package handlers

import (
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/middleware"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
)

// CheckHandler lets a user photograph an item with their phone, e.g. at home,
// to see which bin it belongs in. No bin is involved and no lid opens.
type CheckHandler struct {
	Repo     *repository.CheckRepository
	AIClient *clients.AIClient
	Storage  *storage.FileStorage
}

func NewCheckHandler(repo *repository.CheckRepository, aiClient *clients.AIClient, fileStorage *storage.FileStorage) *CheckHandler {
	return &CheckHandler{Repo: repo, AIClient: aiClient, Storage: fileStorage}
}

type checkRequest struct {
	Image *multipart.FileHeader `form:"image" binding:"required"`
}

// Check: POST /api/check (multipart, field "image"; user session).
// Answers with the category the model sees, like a scan result.
func (h *CheckHandler) Check(c *gin.Context) {
	var req checkRequest
	if err := c.ShouldBind(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": errImageTooLarge.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "image is required"})
		return
	}

	contentType, err := DetectImageType(req.Image)
	if err != nil {
		if errors.Is(err, errImageTooLarge) || errors.Is(err, errUnsupportedImage) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read image"})
		return
	}

	prediction, err := h.AIClient.Predict(req.Image, contentType)
	if err != nil {
		log.Printf("[WARN] AI prediction for a check failed: %v", err)
		prediction = &clients.Prediction{Category: "unknown", Reason: "classifier_unavailable"}
	}

	// Kept for training the model later. The user already has the answer, so
	// a failed upload or save is only logged.
	h.save(c, req.Image, contentType, prediction)

	c.JSON(http.StatusOK, gin.H{
		"category":   prediction.Category,
		"confidence": prediction.Confidence,
		"can_sort":   prediction.CanSort,
		"reason":     prediction.Reason,
	})
}

func (h *CheckHandler) save(c *gin.Context, image *multipart.FileHeader, contentType string, prediction *clients.Prediction) {
	imageURL, _, err := h.Storage.UploadImageTo("checks", image, contentType)
	if err != nil {
		log.Printf("[WARN] could not store check photo: %v", err)
		return
	}
	err = h.Repo.SaveCheck(c.Request.Context(), repository.Check{
		Username:   c.GetString(middleware.UsernameKey),
		ImageURL:   imageURL,
		Category:   prediction.Category,
		Confidence: prediction.Confidence,
		CanSort:    prediction.CanSort,
		Reason:     prediction.Reason,
		CheckedAt:  time.Now().UTC(),
	})
	if err != nil {
		log.Printf("[WARN] could not save check: %v", err)
	}
}
