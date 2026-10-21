package handlers

import (
	"fmt"
	"net/http"

	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/models"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
)

type ScanHandler struct {
	Repo     *repository.ScanRepository
	AIClient *clients.AIClient
	Storage  *storage.FileStorage
}

func NewScanHandler(repo *repository.ScanRepository, aiClient *clients.AIClient, fileStorage *storage.FileStorage) *ScanHandler {
	return &ScanHandler{
		Repo:     repo,
		AIClient: aiClient,
		Storage:  fileStorage,
	}
}

func (h *ScanHandler) HandleScan(c *gin.Context) {
	var req models.ScanRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	imageURL, err := h.Storage.UploadImage(req.Image)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
		return
	}

	detectedCategory, err := h.AIClient.PredictCategory(imageURL)
	if err != nil {
		fmt.Printf("[WARN] AI Prediction failed: %v\n", err)
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