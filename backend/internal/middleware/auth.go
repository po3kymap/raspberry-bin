package middleware

import (
	"crypto/sha256"
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
)

const APIKeyHeader = "X-API-Key"

// APIKeyAuth rejects requests that do not carry the device API key.
// Both keys are hashed first so the comparison takes the same time
func APIKeyAuth(apiKey string) gin.HandlerFunc {
	expected := sha256.Sum256([]byte(apiKey))

	return func(c *gin.Context) {
		provided := sha256.Sum256([]byte(c.GetHeader(APIKeyHeader)))
		if subtle.ConstantTimeCompare(provided[:], expected[:]) != 1 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}
		c.Next()
	}
}
