package repository

import (
	"context"
	"raspberry-bin-api/internal/models"
	"time"

	"cloud.google.com/go/firestore"
)

type ScanRepository struct {
	DB *firestore.Client
}

func NewScanRepository(db *firestore.Client) *ScanRepository {
	return &ScanRepository{DB: db}
}

func (r *ScanRepository) SaveScan(scan models.ScanResponse, imageURL string) error {
	ctx := context.Background()

	data := map[string]interface{}{
		"bin_id":     scan.BinID,
		"category":   scan.Category,
		"image_url":  imageURL,
		"scanned_at": time.Now(),
	}

	_, _, err := r.DB.Collection("scans").Add(ctx, data)
	return err
}