package middleware

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/models"

	"github.com/gin-gonic/gin"
)

// Keys under which RequireSession stores the logged-in user in the Gin context.
const (
	UsernameKey = "username"
	RoleKey     = "role"
)

type SessionValidator interface {
	ValidateSession(ctx context.Context, token string) (*models.Session, error)
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

// RequireSession lets the request through only with a valid session token
// and stores the username and role for the next handlers.
func RequireSession(sessions SessionValidator) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := BearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		session, err := sessions.ValidateSession(c.Request.Context(), token)
		if errors.Is(err, auth.ErrInvalidSession) {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		if err != nil {
			log.Printf("[ERROR] session check failed: %v", err)
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}

		c.Set(UsernameKey, session.Username)
		c.Set(RoleKey, session.Role)
		c.Next()
	}
}

// RequireRole must run after RequireSession. A logged-in user without the
// role gets 403, so a regular user's token never opens admin routes.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetString(RoleKey) != role {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "forbidden"})
			return
		}
		c.Next()
	}
}
