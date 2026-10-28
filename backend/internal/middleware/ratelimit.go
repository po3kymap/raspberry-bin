package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const visitorTTL = 3 * time.Minute

type visitor struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

// RateLimiter keeps a token bucket per client IP.
type RateLimiter struct {
	mu       sync.Mutex
	visitors map[string]*visitor
	limit    rate.Limit
	burst    int
}

func NewRateLimiter(limit rate.Limit, burst int) *RateLimiter {
	rl := &RateLimiter{
		visitors: make(map[string]*visitor),
		limit:    limit,
		burst:    burst,
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) getLimiter(ip string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	v, ok := rl.visitors[ip]
	if !ok {
		v = &visitor{limiter: rate.NewLimiter(rl.limit, rl.burst)}
		rl.visitors[ip] = v
	}
	v.lastSeen = time.Now()
	return v.limiter
}

// cleanup removes clients that have been idle, so the map does not grow forever.
func (rl *RateLimiter) cleanup() {
	for {
		time.Sleep(time.Minute)

		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > visitorTTL {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

// Middleware limits requests per client IP.
func (rl *RateLimiter) Middleware() gin.HandlerFunc {
	return rl.MiddlewareByKey(func(c *gin.Context) string { return c.ClientIP() })
}

// MiddlewareByKey limits requests per key, e.g. per logged-in user, so that
// many users behind one IP (a university network) do not block each other.
func (rl *RateLimiter) MiddlewareByKey(key func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !rl.getLimiter(key(c)).Allow() {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			return
		}
		c.Next()
	}
}

// MaxConcurrentPerIP limits how many requests from one IP may be in progress
// at the same time. Long-polling requests stay open for up to 30 s, so without
// this one client could open thousands of them.
func MaxConcurrentPerIP(limit int) gin.HandlerFunc {
	var mu sync.Mutex
	active := make(map[string]int)

	return func(c *gin.Context) {
		ip := c.ClientIP()

		mu.Lock()
		if active[ip] >= limit {
			mu.Unlock()
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many open requests"})
			return
		}
		active[ip]++
		mu.Unlock()

		defer func() {
			mu.Lock()
			if active[ip]--; active[ip] == 0 {
				delete(active, ip)
			}
			mu.Unlock()
		}()

		c.Next()
	}
}

// MaxBodySize stops reading the request body after n bytes.
func MaxBodySize(n int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, n)
		c.Next()
	}
}
