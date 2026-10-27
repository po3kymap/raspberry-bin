package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"raspberry-bin-api/internal/auth"

	"github.com/gin-gonic/gin"
)

// AdminUsernameKey is where RequireAdmin stores the logged-in admin's username in the Gin context.
const AdminUsernameKey = "admin_username"

type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (string, error)
}

// BearerToken extracts the token from an "Authorization: Bearer <token>" header.
func BearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}

// RequireAdmin lets the request through only with a valid admin session token.
// It runs on every protected request, as required by the RAD.
func RequireAdmin(sessions SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := BearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		username, err := sessions.ValidateSession(c.Request.Context(), token)
		if errors.Is(err, auth.ErrInvalidSession) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if err != nil {
			log.Printf("[ERROR] session check failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.Set(AdminUsernameKey, username)
		c.Next()
	}
}
