package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"raspberry-bin-api/internal/auth"

	"github.com/gin-gonic/gin"
)

const validToken = "valid-session-token"

type fakeSessions struct{}

func (fakeSessions) ValidateSession(_ context.Context, token string) (string, error) {
	switch token {
	case validToken:
		return "admin", nil
	case "database-down":
		return "", errors.New("connection refused")
	default:
		return "", auth.ErrInvalidSession
	}
}

func TestRequireAdmin(t *testing.T) {
	r := gin.New()
	r.GET("/api/admin/me", RequireAdmin(fakeSessions{}), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(AdminUsernameKey))
	})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"no header", "", http.StatusUnauthorized},
		{"not a bearer token", "Basic YWRtaW46cGFzcw==", http.StatusUnauthorized},
		{"empty bearer token", "Bearer ", http.StatusUnauthorized},
		{"unknown token", "Bearer stolen-or-made-up", http.StatusUnauthorized},
		{"session store error", "Bearer database-down", http.StatusInternalServerError},
		{"valid token", "Bearer " + validToken, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/admin/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusOK && rec.Body.String() != "admin" {
				t.Errorf("username in context = %q, want %q", rec.Body.String(), "admin")
			}
		})
	}
}
