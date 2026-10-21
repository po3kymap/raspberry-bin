package handlers

import (
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/models"
	"raspberry-bin-api/internal/repository"

	"github.com/gin-gonic/gin"
)

type ScanHandler struct {
	Repo     *repository.ScanRepository
	AIClient *clients.AIClient
}

func NewScanHandler(repo *repository.ScanRepository, aiClient *clients.AIClient) *ScanHandler {
	return &ScanHandler{
		Repo:     repo,
		AIClient: aiClient,
	}
}

func (h *ScanHandler) HandleScan(c *gin.Context) {
	var req models.ScanRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filename := fmt.Sprintf("%d-%s", time.Now().UnixNano(), filepath.Base(req.Image.Filename))
	savePath := filepath.Join("uploads", filename)

	if err := c.SaveUploadedFile(req.Image, savePath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	imageURL := fmt.Sprintf("http://localhost:8080/uploads/%s", filename)

	detectedCategory, err := h.AIClient.PredictCategory(imageURL)
	if err != nil {
		fmt.Printf("[WARN] AI Prediction failed (using fallback): %v\n", err)
		detectedCategory = "unknown"
	}

	resp := models.ScanResponse{
		BinID:    req.BinID,
		Category: detectedCategory,
	}

	err = h.Repo.SaveScan(resp, imageURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to database"})
		return
	}

	c.JSON(http.StatusOK, resp)
}