package handlers

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"raspberry-bin-api/internal/middleware"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/stats"

	"github.com/gin-gonic/gin"
)

const (
	defaultScanPageSize = 20
	maxScanPageSize     = 50

	defaultStatsDays = 30
	maxStatsDays     = 366
)

// MeHandler serves the logged-in user's own data. The username always comes
// from the session, never from the request, so nobody can read another
// user's history.
type MeHandler struct {
	ScanRepo *repository.ScanRepository
	Users    *repository.UserRepository
	Location *time.Location
}

func NewMeHandler(scans *repository.ScanRepository, users *repository.UserRepository, loc *time.Location) *MeHandler {
	return &MeHandler{ScanRepo: scans, Users: users, Location: loc}
}

// Scans: GET /api/me/scans?limit=20&before=<RFC 3339 time>, newest first.
// To load the next page, pass the scanned_at of the last scan as before.
func (h *MeHandler) Scans(c *gin.Context) {
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

	scans, err := h.ScanRepo.ListUserScans(c.Request.Context(), c.GetString(middleware.UsernameKey), limit, before)
	if err != nil {
		log.Printf("[ERROR] listing scans failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, scans)
}

// Stats: GET /api/me/stats?from=2026-10-01&to=2026-10-28 (dates in Dutch time,
// both included). Defaults to the last 30 days.
func (h *MeHandler) Stats(c *gin.Context) {
	today := time.Now().In(h.Location).Format(stats.DateLayout)

	from, to, ok := parseRange(c, h.Location, today)
	if !ok {
		return
	}

	username := c.GetString(middleware.UsernameKey)
	scans, err := h.ScanRepo.UserScansBetween(c.Request.Context(), username, from, to)
	if err != nil {
		log.Printf("[ERROR] reading scans for stats failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	user, err := h.Users.GetUser(c.Request.Context(), username)
	if err != nil {
		log.Printf("[ERROR] reading user for stats failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary": stats.Summarize(scans, h.Location),
		"streak": gin.H{
			"current":          stats.CurrentStreak(user, today),
			"longest":          user.LongestStreak,
			"last_sorted_date": user.LastSortedDate,
		},
	})
}

// parseRange turns ?from and ?to (dates in loc) into [start of from, start of the day after to).
// Used by the user and the admin statistics.
func parseRange(c *gin.Context, loc *time.Location, today string) (time.Time, time.Time, bool) {
	parse := func(name, fallback string) (time.Time, bool) {
		value := c.DefaultQuery(name, fallback)
		t, err := time.ParseInLocation(stats.DateLayout, value, loc)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid " + name + ", use YYYY-MM-DD"})
			return time.Time{}, false
		}
		return t, true
	}

	to, ok := parse("to", today)
	if !ok {
		return time.Time{}, time.Time{}, false
	}
	from, ok := parse("from", to.AddDate(0, 0, -(defaultStatsDays-1)).Format(stats.DateLayout))
	if !ok {
		return time.Time{}, time.Time{}, false
	}

	end := to.AddDate(0, 0, 1)
	if from.After(to) || end.Sub(from) > maxStatsDays*24*time.Hour {
		c.JSON(http.StatusBadRequest, gin.H{"error": "from must be before to, at most 366 days"})
		return time.Time{}, time.Time{}, false
	}
	return from, end, true
}
