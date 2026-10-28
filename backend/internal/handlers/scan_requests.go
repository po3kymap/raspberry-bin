package handlers

import (
	"errors"
	"log"
	"net/http"
	"strconv"
	"time"

	"raspberry-bin-api/internal/bins"
	"raspberry-bin-api/internal/dispatch"
	"raspberry-bin-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Longest a long-polling request may wait before the server answers anyway
const maxWait = 30 * time.Second

// parseWait reads ?wait=<seconds>, capped at maxWait. No value means "answer now".
func parseWait(c *gin.Context) time.Duration {
	seconds, err := strconv.Atoi(c.Query("wait"))
	if err != nil || seconds <= 0 {
		return 0
	}
	wait := time.Duration(seconds) * time.Second
	if wait > maxWait {
		return maxWait
	}
	return wait
}

type ScanRequestHandler struct {
	Dispatcher *dispatch.Dispatcher
	Bins       *bins.Service
}

func NewScanRequestHandler(dispatcher *dispatch.Dispatcher, binService *bins.Service) *ScanRequestHandler {
	return &ScanRequestHandler{Dispatcher: dispatcher, Bins: binService}
}

type createScanRequest struct {
	BinID string `json:"bin_id" binding:"required"`
	// The secret code from the QR on the bin; proves the user is at the bin
	Code string `json:"code" binding:"required"`
}

// Create: POST /api/scan-requests {"bin_id": "...", "code": "..."} (user session)
func (h *ScanRequestHandler) Create(c *gin.Context) {
	var req createScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bin_id and code are required"})
		return
	}
	username := c.GetString(middleware.UsernameKey)

	err := h.Bins.VerifyCode(c.Request.Context(), req.BinID, req.Code)
	if errors.Is(err, bins.ErrInvalidBinCode) {
		log.Printf("[AUDIT] invalid bin code for %q from user %q (%s)", req.BinID, username, c.ClientIP())
		c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Printf("[ERROR] bin code check failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	requestID, err := h.Dispatcher.CreateScanRequest(req.BinID, username)
	if errors.Is(err, dispatch.ErrBinBusy) || errors.Is(err, dispatch.ErrUserBusy) {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Printf("[ERROR] creating scan request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"request_id": requestID})
}

// Get: GET /api/scan-requests/{id}?wait=25 (user session, own requests only)
func (h *ScanRequestHandler) Get(c *gin.Context) {
	requestID := c.Param("id")
	if !requestIDPattern.MatchString(requestID) {
		c.JSON(http.StatusNotFound, gin.H{"error": dispatch.ErrRequestNotFound.Error()})
		return
	}

	snapshot, err := h.Dispatcher.Wait(c.Request.Context(), requestID, c.GetString(middleware.UsernameKey), parseWait(c))
	if errors.Is(err, dispatch.ErrRequestNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	if err != nil {
		log.Printf("[ERROR] reading scan request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}
	c.JSON(http.StatusOK, snapshot)
}
