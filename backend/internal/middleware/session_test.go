package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/models"

	"github.com/gin-gonic/gin"
)

const (
	adminToken = "valid-admin-token"
	userToken  = "valid-user-token"
)

type fakeSessions struct{}

func (fakeSessions) ValidateSession(_ context.Context, token string) (*models.Session, error) {
	switch token {
	case adminToken:
		return &models.Session{Username: "admin", Role: models.RoleAdmin}, nil
	case userToken:
		return &models.Session{Username: "alice", Role: models.RoleUser}, nil
	case "database-down":
		return nil, errors.New("connection refused")
	default:
		return nil, auth.ErrInvalidSession
	}
}

func request(r *gin.Engine, path, header string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("GET", path, nil)
	if header != "" {
		req.Header.Set("Authorization", header)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestRequireSession(t *testing.T) {
	r := gin.New()
	r.GET("/api/auth/me", RequireSession(fakeSessions{}), func(c *gin.Context) {
		c.String(http.StatusOK, c.GetString(UsernameKey)+"/"+c.GetString(RoleKey))
	})

	tests := []struct {
		name   string
		header string
		want   int
		body   string
	}{
		{"no header", "", http.StatusUnauthorized, ""},
		{"not a bearer token", "Basic YWRtaW46cGFzcw==", http.StatusUnauthorized, ""},
		{"empty bearer token", "Bearer ", http.StatusUnauthorized, ""},
		{"unknown token", "Bearer stolen-or-made-up", http.StatusUnauthorized, ""},
		{"session store error", "Bearer database-down", http.StatusInternalServerError, ""},
		{"user token", "Bearer " + userToken, http.StatusOK, "alice/user"},
		{"admin token", "Bearer " + adminToken, http.StatusOK, "admin/admin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := request(r, "/api/auth/me", tt.header)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.body != "" && rec.Body.String() != tt.body {
				t.Errorf("context = %q, want %q", rec.Body.String(), tt.body)
			}
		})
	}
}

// A regular user's valid token must never open admin routes.
func TestRequireRoleAdmin(t *testing.T) {
	r := gin.New()
	r.GET("/api/admin/me", RequireSession(fakeSessions{}), RequireRole(models.RoleAdmin), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"no token", "", http.StatusUnauthorized},
		{"user token", "Bearer " + userToken, http.StatusForbidden},
		{"admin token", "Bearer " + adminToken, http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := request(r, "/api/admin/me", tt.header).Code; got != tt.want {
				t.Errorf("status = %d, want %d", got, tt.want)
			}
		})
	}
}
