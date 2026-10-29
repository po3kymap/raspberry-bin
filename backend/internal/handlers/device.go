package handlers

import (
	"errors"
	"log"
	"net/http"

	"raspberry-bin-api/internal/activity"
	"raspberry-bin-api/internal/bins"
	"raspberry-bin-api/internal/dispatch"
	"raspberry-bin-api/internal/middleware"

	"github.com/gin-gonic/gin"
)

// Compartments the admin can open or close
var compartments = map[string]bool{
	"GENERAL_WASTE": true,
	"ORGANIC":       true,
	"PAPER":         true,
	"PMD":           true,
}

type DeviceHandler struct {
	Dispatcher *dispatch.Dispatcher
	Bins       *bins.Service
	Activity   *activity.Recorder
}

func NewDeviceHandler(dispatcher *dispatch.Dispatcher, binService *bins.Service, recorder *activity.Recorder) *DeviceHandler {
	return &DeviceHandler{Dispatcher: dispatcher, Bins: binService, Activity: recorder}
}

func (h *DeviceHandler) checkBin(c *gin.Context, binID string) bool {
	err := h.Bins.Exists(c.Request.Context(), binID)
	switch {
	case errors.Is(err, bins.ErrInvalidBinID), errors.Is(err, bins.ErrBinNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": "bin not found"})
		return false
	case err != nil:
		log.Printf("[ERROR] bin lookup failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return false
	}
	return true
}

// Commands: GET /api/device/commands?bin_id=bin-27&wait=25 (device API key).
// Answers at once when a command is queued; 204 when there was none in time.
func (h *DeviceHandler) Commands(c *gin.Context) {
	binID := c.Query("bin_id")
	if !h.checkBin(c, binID) {
		return
	}

	cmd, ok := h.Dispatcher.NextCommand(c.Request.Context(), binID, parseWait(c))
	if !ok {
		c.Status(http.StatusNoContent)
		return
	}
	c.JSON(http.StatusOK, cmd)
}

type lidRequest struct {
	BinID string `json:"bin_id" binding:"required"`
}

// Lid: POST /api/admin/lids/{compartment}/{open|close} {"bin_id": "..."} (admin).
// Queues the command for the Pi; 202 means queued, not yet done.
func (h *DeviceHandler) Lid(c *gin.Context) {
	compartment, action := c.Param("compartment"), c.Param("action")
	if !compartments[compartment] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unknown compartment"})
		return
	}

	var command string
	switch action {
	case "open":
		command = dispatch.CommandOpenLid
	case "close":
		command = dispatch.CommandCloseLid
	default:
		c.JSON(http.StatusNotFound, gin.H{"error": "unknown action"})
		return
	}

	var req lidRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bin_id is required"})
		return
	}
	if !h.checkBin(c, req.BinID) {
		return
	}

	h.Dispatcher.EnqueueCommand(req.BinID, dispatch.Command{Command: command, Compartment: compartment})

	log.Printf("[AUDIT] admin %q: %s %s on %q from %s",
		c.GetString(middleware.UsernameKey), action, compartment, req.BinID, c.ClientIP())
	h.Activity.Record(c.Request.Context(), activity.Event{
		Type: activity.Lid, Username: c.GetString(middleware.UsernameKey), BinID: req.BinID,
		Compartment: compartment, Action: action, IP: c.ClientIP(),
	})
	c.JSON(http.StatusAccepted, gin.H{"status": "queued"})
}
