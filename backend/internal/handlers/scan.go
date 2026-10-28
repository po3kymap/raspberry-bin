package handlers

import (
	"errors"
	"log"
	"mime/multipart"
	"net/http"
	"regexp"
	"time"

	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/dispatch"
	"raspberry-bin-api/internal/models"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
)

const MaxImageSize = 5 << 20 // 5 MB

var (
	binIDPattern     = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
	requestIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

	allowedImageTypes = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}

	errInvalidBinID     = errors.New("invalid bin_id")
	errImageTooLarge    = errors.New("image is too large")
	errUnsupportedImage = errors.New("image must be JPEG or PNG")
)

type ScanHandler struct {
	Repo       *repository.ScanRepository
	AIClient   *clients.AIClient
	Storage    *storage.FileStorage
	Dispatcher *dispatch.Dispatcher
}

func NewScanHandler(repo *repository.ScanRepository, aiClient *clients.AIClient, fileStorage *storage.FileStorage, dispatcher *dispatch.Dispatcher) *ScanHandler {
	return &ScanHandler{
		Repo:       repo,
		AIClient:   aiClient,
		Storage:    fileStorage,
		Dispatcher: dispatcher,
	}
}

func ValidateBinID(binID string) error {
	if !binIDPattern.MatchString(binID) {
		return errInvalidBinID
	}
	return nil
}

// DetectImageType checks the file size and looks at the file's first bytes
// to find its real type, instead of trusting the name or header sent by the client.
func DetectImageType(file *multipart.FileHeader) (string, error) {
	if file.Size <= 0 || file.Size > MaxImageSize {
		return "", errImageTooLarge
	}

	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	head := make([]byte, 512)
	n, err := f.Read(head)
	if err != nil && n == 0 {
		return "", err
	}

	contentType := http.DetectContentType(head[:n])
	if !allowedImageTypes[contentType] {
		return "", errUnsupportedImage
	}
	return contentType, nil
}

func (h *ScanHandler) HandleScan(c *gin.Context) {
	var req models.ScanRequest
	if err := c.ShouldBind(&req); err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": errImageTooLarge.Error()})
			return
		}
		log.Printf("[WARN] invalid scan request from %s: %v", c.ClientIP(), err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "bin_id, request_id and image are required"})
		return
	}

	if err := ValidateBinID(req.BinID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if !requestIDPattern.MatchString(req.RequestID) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request_id"})
		return
	}

	contentType, err := DetectImageType(req.Image)
	if err != nil {
		if errors.Is(err, errImageTooLarge) || errors.Is(err, errUnsupportedImage) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		log.Printf("[ERROR] failed to read uploaded image: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "could not read image"})
		return
	}

	// Only photos for a scan a logged-in user started at this bin are accepted,
	// and each request ID only once. This happens before anything is stored.
	username, err := h.Dispatcher.Claim(req.RequestID, req.BinID)
	switch {
	case errors.Is(err, dispatch.ErrRequestNotFound):
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	case errors.Is(err, dispatch.ErrWrongBin):
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	case errors.Is(err, dispatch.ErrRequestNotActive):
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	case err != nil:
		log.Printf("[ERROR] claiming scan request failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	imageURL, err := h.Storage.UploadImage(req.Image, contentType)
	if err != nil {
		log.Printf("[ERROR] image upload failed: %v", err)
		h.Dispatcher.Fail(req.RequestID, "internal_error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload image"})
		return
	}

	prediction, err := h.AIClient.Predict(req.Image, contentType)
	if err != nil {
		// Fail closed: without a classification no lid may open
		log.Printf("[WARN] AI prediction failed: %v", err)
		prediction = &clients.Prediction{Category: "unknown", Reason: "classifier_unavailable"}
	}

	result := models.ScanResult{
		Category:   prediction.Category,
		Confidence: prediction.Confidence,
		CanSort:    prediction.CanSort,
		Reason:     prediction.Reason,
		BinID:      req.BinID,
		ScannedAt:  time.Now().UTC(),
	}

	err = h.Repo.SaveScan(c.Request.Context(), repository.StoredScan{
		ScanResult: result,
		Username:   username,
		RequestID:  req.RequestID,
		ImageURL:   imageURL,
	})
	if err != nil {
		log.Printf("[ERROR] saving scan failed: %v", err)
		h.Dispatcher.Fail(req.RequestID, "internal_error")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to database"})
		return
	}

	h.Dispatcher.Complete(req.RequestID, result)

	c.JSON(http.StatusOK, models.ScanResponse{
		BinID:      req.BinID,
		Category:   result.Category,
		Confidence: result.Confidence,
		CanSort:    result.CanSort,
		Reason:     result.Reason,
	})
}
