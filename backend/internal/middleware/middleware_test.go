package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const testAPIKey = "test-key-0123456789abcdef0123456789abcdef"

func init() {
	gin.SetMode(gin.TestMode)
}

func newRouter(handlers ...gin.HandlerFunc) *gin.Engine {
	r := gin.New()
	handlers = append(handlers, func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	r.POST("/api/scan", handlers...)
	return r
}

func doRequest(r *gin.Engine, apiKey string) int {
	req := httptest.NewRequest("POST", "/api/scan", nil)
	if apiKey != "" {
		req.Header.Set(APIKeyHeader, apiKey)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec.Code
}

func TestAPIKeyAuth(t *testing.T) {
	r := newRouter(APIKeyAuth(testAPIKey))

	tests := []struct {
		name   string
		apiKey string
		want   int
	}{
		{"no key", "", http.StatusUnauthorized},
		{"wrong key", "wrong-key", http.StatusUnauthorized},
		{"key prefix only", testAPIKey[:10], http.StatusUnauthorized},
		{"correct key", testAPIKey, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := doRequest(r, tt.apiKey); got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRateLimiterBlocksAfterBurst(t *testing.T) {
	limiter := NewRateLimiter(rate.Every(time.Hour), 3)
	r := newRouter(limiter.Middleware())

	for i := 0; i < 3; i++ {
		if got := doRequest(r, ""); got != http.StatusOK {
			t.Fatalf("request %d: status = %d, want %d", i+1, got, http.StatusOK)
		}
	}
	if got := doRequest(r, ""); got != http.StatusTooManyRequests {
		t.Errorf("request 4: status = %d, want %d", got, http.StatusTooManyRequests)
	}
}

func TestRateLimiterByKeySeparatesUsers(t *testing.T) {
	limiter := NewRateLimiter(rate.Every(time.Hour), 1)
	r := gin.New()
	r.POST("/api/scan", func(c *gin.Context) {
		c.Set(UsernameKey, c.GetHeader("X-User"))
	}, limiter.MiddlewareByKey(func(c *gin.Context) string { return c.GetString(UsernameKey) }), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	send := func(user string) int {
		req := httptest.NewRequest("POST", "/api/scan", nil)
		req.Header.Set("X-User", user)
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec.Code
	}

	// Same IP (httptest), different users: each has their own budget
	if send("alice") != http.StatusOK || send("bob") != http.StatusOK {
		t.Error("first request of each user should pass")
	}
	if got := send("alice"); got != http.StatusTooManyRequests {
		t.Errorf("alice's second request: status = %d, want 429", got)
	}
}

func TestMaxConcurrentPerIP(t *testing.T) {
	release := make(chan struct{})
	r := gin.New()
	r.GET("/wait", MaxConcurrentPerIP(2), func(c *gin.Context) {
		<-release
		c.Status(http.StatusOK)
	})

	send := func(codes chan<- int) {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest("GET", "/wait", nil))
		codes <- rec.Code
	}

	codes := make(chan int, 3)
	go send(codes)
	go send(codes)
	time.Sleep(50 * time.Millisecond) // both are now waiting

	go send(codes)
	if got := <-codes; got != http.StatusTooManyRequests {
		t.Errorf("third open request: status = %d, want 429", got)
	}

	close(release)
	if <-codes != http.StatusOK || <-codes != http.StatusOK {
		t.Error("the first two requests should succeed")
	}
}
