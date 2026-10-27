package handlers

import (
	"errors"
	"log"
	"net/http"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

type AdminHandler struct {
	Auth *auth.Service
}

func NewAdminHandler(authService *auth.Service) *AdminHandler {
	return &AdminHandler{Auth: authService}
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Login: POST /api/admin/login {"username": "...", "password": "..."}
func (h *AdminHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	token, expiresAt, err := h.Auth.Login(c.Request.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		log.Printf("[AUDIT] failed admin login for %q from %s", req.Username, c.ClientIP())
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	case errors.Is(err, auth.ErrAccountLocked):
		log.Printf("[AUDIT] locked admin login attempt for %q from %s", req.Username, c.ClientIP())
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	case err != nil:
		log.Printf("[ERROR] admin login failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] admin %q logged in from %s", req.Username, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{
		"token":      token,
		"expires_at": expiresAt,
	})
}

// Logout: POST /api/admin/logout (needs a valid session)
func (h *AdminHandler) Logout(c *gin.Context) {
	token, _ := middleware.BearerToken(c.GetHeader("Authorization"))
	if err := h.Auth.Logout(c.Request.Context(), token); err != nil {
		log.Printf("[ERROR] admin logout failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] admin %q logged out", c.GetString(middleware.AdminUsernameKey))
	c.Status(http.StatusNoContent)
}

// Me: GET /api/admin/me returns who is logged in; useful for the dashboard
// to check whether its stored token is still valid.
func (h *AdminHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"username": c.GetString(middleware.AdminUsernameKey)})
}
