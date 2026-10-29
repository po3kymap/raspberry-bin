package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"raspberry-bin-api/internal/activity"
	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/bins"
	"raspberry-bin-api/internal/middleware"
	"raspberry-bin-api/internal/models"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/stats"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
)

// AdminHandler serves the admin screens: user management, connecting to a bin
// with its QR code and the statistics. The routes are behind RequireRole(admin).
type AdminHandler struct {
	Auth     *auth.Service
	Users    *repository.UserRepository
	ScanRepo *repository.ScanRepository
	Bins     *bins.Service
	Activity *activity.Recorder
	Events   *repository.ActivityRepository
	Storage  *storage.FileStorage
	Location *time.Location
}

func NewAdminHandler(authService *auth.Service, users *repository.UserRepository, scans *repository.ScanRepository, binService *bins.Service, recorder *activity.Recorder, events *repository.ActivityRepository, fileStorage *storage.FileStorage, loc *time.Location) *AdminHandler {
	return &AdminHandler{Auth: authService, Users: users, ScanRepo: scans, Bins: binService, Activity: recorder, Events: events, Storage: fileStorage, Location: loc}
}

// userInfo is what admins see about a user; it never contains the password hash.
type userInfo struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
	// null if the user has not logged in since this was added
	LastLoginAt    *time.Time `json:"last_login_at"`
	Locked         bool       `json:"locked"`
	CurrentStreak  int        `json:"current_streak"`
	LongestStreak  int        `json:"longest_streak"`
	LastSortedDate string     `json:"last_sorted_date"`
	TotalScans     int        `json:"total_scans"`
	SortedScans    int        `json:"sorted_scans"`
}

// ListUsers: GET /api/admin/users
func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.Users.ListUsers(c.Request.Context())
	if err != nil {
		log.Printf("[ERROR] listing users failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	scanCounts, err := h.ScanRepo.CountScansByUser(c.Request.Context())
	if err != nil {
		log.Printf("[ERROR] counting scans per user failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	now := time.Now()
	today := now.In(h.Location).Format(stats.DateLayout)
	list := make([]userInfo, 0, len(users))
	for i := range users {
		u := &users[i]
		info := userInfo{
			Username:       u.Username,
			Role:           u.Role,
			CreatedAt:      u.CreatedAt,
			Locked:         now.Before(u.LockedUntil),
			CurrentStreak:  stats.CurrentStreak(u, today),
			LongestStreak:  u.LongestStreak,
			LastSortedDate: u.LastSortedDate,
			TotalScans:     scanCounts[u.Username].Total,
			SortedScans:    scanCounts[u.Username].Sorted,
		}
		if !u.LastLoginAt.IsZero() {
			info.LastLoginAt = &u.LastLoginAt
		}
		list = append(list, info)
	}
	c.JSON(http.StatusOK, list)
}

type createUserRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	// "user" (default) or "admin"
	Role string `json:"role"`
}

// CreateUser: POST /api/admin/users {"username": "...", "password": "...", "role": "user"}
func (h *AdminHandler) CreateUser(c *gin.Context) {
	var req createUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username and password are required"})
		return
	}
	role := req.Role
	if role == "" {
		role = models.RoleUser
	}

	username, err := h.Auth.CreateUser(c.Request.Context(), req.Username, req.Password, role)
	switch {
	case errors.Is(err, auth.ErrInvalidUsername), errors.Is(err, auth.ErrWeakPassword), errors.Is(err, auth.ErrInvalidRole):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, auth.ErrUserExists):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case err != nil:
		log.Printf("[ERROR] creating user failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] admin %q created %s %q from %s",
		c.GetString(middleware.UsernameKey), role, username, c.ClientIP())
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.UserCreated, Username: c.GetString(middleware.UsernameKey), Target: username, Role: role, IP: c.ClientIP(),
	})
	c.JSON(http.StatusCreated, gin.H{"username": username, "role": role})
}

// DeleteUser: DELETE /api/admin/users/{username}. The user's sessions end at once.
func (h *AdminHandler) DeleteUser(c *gin.Context) {
	username := strings.ToLower(strings.TrimSpace(c.Param("username")))
	admin := c.GetString(middleware.UsernameKey)

	// Otherwise an admin could lock themselves out by mistake
	if username == admin {
		c.JSON(http.StatusBadRequest, gin.H{"error": "you cannot delete your own account"})
		return
	}

	err := h.Users.DeleteUser(c.Request.Context(), username)
	switch {
	case errors.Is(err, auth.ErrUserNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	case errors.Is(err, repository.ErrLastAdmin):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case err != nil:
		log.Printf("[ERROR] deleting user %q failed: %v", username, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] admin %q deleted user %q from %s", admin, username, c.ClientIP())
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.UserDeleted, Username: admin, Target: username, IP: c.ClientIP(),
	})
	c.Status(http.StatusNoContent)
}

type connectBinRequest struct {
	BinID string `json:"bin_id" binding:"required"`
	Code  string `json:"code" binding:"required"`
}

// ConnectBin: POST /api/admin/bins/connect {"bin_id": "...", "code": "..."}.
// The admin scans the QR on the bin; a valid code proves they are at that bin.
func (h *AdminHandler) ConnectBin(c *gin.Context) {
	var req connectBinRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bin_id and code are required"})
		return
	}

	err := h.Bins.VerifyCode(c.Request.Context(), req.BinID, req.Code)
	if errors.Is(err, bins.ErrInvalidBinCode) {
		log.Printf("[AUDIT] invalid bin code for %q from admin %q (%s)", req.BinID, c.GetString(middleware.UsernameKey), c.ClientIP())
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Printf("[ERROR] bin code check failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	log.Printf("[AUDIT] admin %q connected to bin %q from %s", c.GetString(middleware.UsernameKey), req.BinID, c.ClientIP())
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.BinConnect, Username: c.GetString(middleware.UsernameKey), BinID: req.BinID, IP: c.ClientIP(),
	})
	c.JSON(http.StatusOK, gin.H{"bin_id": req.BinID})
}

// Stats: GET /api/admin/stats?bin_id=bin-27&from=2026-10-01&to=2026-10-28.
// Without bin_id the statistics of all bins. Dates in Dutch time, both
// included, default last 30 days. The user counts are always for all users.
func (h *AdminHandler) Stats(c *gin.Context) {
	today := time.Now().In(h.Location).Format(stats.DateLayout)

	binID := c.Query("bin_id")
	if binID != "" && bins.ValidateBinID(binID) != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bin_id"})
		return
	}

	from, to, ok := parseRange(c, h.Location, today)
	if !ok {
		return
	}

	scans, err := h.ScanRepo.ScansBetween(c.Request.Context(), binID, from, to)
	if err != nil {
		log.Printf("[ERROR] reading scans for admin stats failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	users, err := h.Users.CountUsers(c.Request.Context(), from, to)
	if err != nil {
		log.Printf("[ERROR] counting users failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": stats.SummarizeOverview(scans, h.Location),
		"users":   users,
	})
}

// ListActivity: GET /api/admin/activity?limit=20&before=<RFC 3339 time>, newest first.
// To load the next page, pass the "at" of the last event as before.
func (h *AdminHandler) ListActivity(c *gin.Context) {
	limit := defaultScanPageSize
	if value := c.Query("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid limit"})
			return
		}
		limit = min(n, maxScanPageSize)
	}

	var before time.Time
	if value := c.Query("before"); value != "" {
		t, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before, use RFC 3339"})
			return
		}
		before = t
	}

	events, err := h.Events.ListEvents(c.Request.Context(), limit, before)
	if err != nil {
		log.Printf("[ERROR] listing activity failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, events)
}

// ScanImage: GET /api/admin/images/{file}, e.g. /api/admin/images/5f1c….jpg for
// the image_id "scans/5f1c….jpg" from the activity log. Admins only; the
// photo is streamed through the backend, so the bucket stays private.
func (h *AdminHandler) ScanImage(c *gin.Context) {
	reader, contentType, err := h.Storage.OpenImage(c.Request.Context(), "scans/"+c.Param("file"))
	switch {
	case errors.Is(err, storage.ErrInvalidImageName), errors.Is(err, storage.ErrImageNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "image not found"})
		return
	case err != nil:
		log.Printf("[ERROR] reading scan image failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	defer reader.Close()

	if contentType != "image/jpeg" && contentType != "image/png" {
		contentType = "application/octet-stream"
	}
	c.Header("Cache-Control", "private, max-age=3600")
	c.DataFromReader(http.StatusOK, -1, contentType, reader, nil)
}
