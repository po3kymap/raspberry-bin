package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"raspberry-bin-api/internal/activity"
	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/middleware"
	"raspberry-bin-api/internal/models"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Auth     *auth.Service
	Activity *activity.Recorder
}

func NewAuthHandler(authService *auth.Service, recorder *activity.Recorder) *AuthHandler {
	return &AuthHandler{Auth: authService, Activity: recorder}
}

type credentials struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Register: POST /api/auth/register {"username": "...", "password": "..."}
func (h *AuthHandler) Register(c *gin.Context) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	username, err := h.Auth.Register(c.Request.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrWeakPassword):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, auth.ErrUserExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case err != nil:
		log.Printf("[ERROR] registration failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] user %q registered from %s", username, c.ClientIP())
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.Register, Username: username, Role: models.RoleUser, IP: c.ClientIP(),
	})
	c.JSON(http.StatusCreated, gin.H{"username": username})
}

// Login: POST /api/auth/login, for users and admins. The role in the answer
// tells the web app which screens to show.
func (h *AuthHandler) Login(c *gin.Context) {
	h.login(c, "")
}

// AdminLogin: POST /api/admin/login, same as Login but only for admins.
func (h *AuthHandler) AdminLogin(c *gin.Context) {
	h.login(c, models.RoleAdmin)
}

func (h *AuthHandler) login(c *gin.Context, requiredRole string) {
	var req credentials
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}

	username := strings.ToLower(strings.TrimSpace(req.Username))
	failed := func(reason string) {
		h.Activity.Record(c.Request.Context(), activity.Event{
			Type: activity.LoginFailed, Username: username, Reason: reason, IP: c.ClientIP(),
		})
	}

	result, err := h.Auth.Login(c.Request.Context(), req.Username, req.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		log.Printf("[AUDIT] failed login for %q from %s", req.Username, c.ClientIP())
		failed("wrong_password")
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	case errors.Is(err, auth.ErrAccountLocked):
		log.Printf("[AUDIT] locked login attempt for %q from %s", req.Username, c.ClientIP())
		failed("locked")
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	case err != nil:
		log.Printf("[ERROR] login failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	if requiredRole != "" && result.Role != requiredRole {
		// Correct password but not an admin: end the new session and answer
		// like a wrong password, so the admin login reveals nothing extra
		_ = h.Auth.Logout(c.Request.Context(), result.Token)
		log.Printf("[AUDIT] non-admin %q tried the admin login from %s", req.Username, c.ClientIP())
		failed("not_admin")
		c.JSON(http.StatusUnauthorized, gin.H{"error": auth.ErrInvalidCredentials.Error()})
		return
	}

	log.Printf("[AUDIT] %s %q logged in from %s", result.Role, req.Username, c.ClientIP())
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.Login, Username: username, Role: result.Role, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, gin.H{
		"token":      result.Token,
		"expires_at": result.ExpiresAt,
		"role":       result.Role,
	})
}

// Logout: POST /api/auth/logout (needs a valid session)
func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := middleware.BearerToken(c.GetHeader("Authorization"))
	if err := h.Auth.Logout(c.Request.Context(), token); err != nil {
		log.Printf("[ERROR] logout failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] %q logged out", c.GetString(middleware.UsernameKey))
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.Logout, Username: c.GetString(middleware.UsernameKey), Role: c.GetString(middleware.RoleKey),
	})
	c.Status(http.StatusNoContent)
}

// Me: GET /api/auth/me returns who is logged in; the web app uses it to check
// whether its stored token is still valid.
func (h *AuthHandler) Me(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"username": c.GetString(middleware.UsernameKey),
		"role":     c.GetString(middleware.RoleKey),
	})
}
