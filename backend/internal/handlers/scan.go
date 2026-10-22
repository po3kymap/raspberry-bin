package handlers

import (
	"errors"
	"fmt"
	"log"
	"mime/multipart"
	"net/http"
	"regexp"

	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/models"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
)

const MaxImageSize = 5 << 20 // 5 MB

var (
	binIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)

	allowedImageTypes = map[string]bool{
		"image/jpeg": true,
		"image/png":  true,
	}

	errInvalidBinID     = errors.New("invalid bin_id")
	errImageTooLarge    = errors.New("image is too large")
	errUnsupportedImage = errors.New("image must be JPEG or PNG")
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
		c.JSON(http.StatusBadRequest, gin.H{"error": "bin_id and image are required"})
		return
	}

	if err := ValidateBinID(req.BinID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
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

	imageURL, err := h.Storage.UploadImage(req.Image, contentType)
	if err != nil {
		log.Printf("[ERROR] image upload failed: %v", err)
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
		log.Printf("[ERROR] saving scan failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save to database"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
