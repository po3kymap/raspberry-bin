package handlers

import (
	"net/http"
	"raspberry-bin-api/internal/models"

	"cloud.google.com/go/firestore"
	"github.com/gin-gonic/gin"
)

type ScanHandler struct {
	DB *firestore.Client
}

func NewScanHandler(db *firestore.Client) *ScanHandler {
	return &ScanHandler{DB: db}
}

func (h *ScanHandler) HandleScan(c *gin.Context) {
	var req models.ScanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// ML integration placeholder
	detectedCategory := "plastic"
	confidence := 0.95

	resp := models.ScanResponse{
		BinID:      req.BinID,
		Category:   detectedCategory,
		Confidence: confidence,
	}

	c.JSON(http.StatusOK, resp)
}