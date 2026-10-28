package models

import (
	"mime/multipart"
	"time"
)

// ScanRequest is the form the Pi sends to POST /api/scan after a "capture" command.
type ScanRequest struct {
	BinID     string                `form:"bin_id" binding:"required"`
	RequestID string                `form:"request_id" binding:"required"`
	Image     *multipart.FileHeader `form:"image" binding:"required"`
}

// ScanResponse tells the Pi what to do: open the lid for Category only if CanSort is true.
type ScanResponse struct {
	BinID      string  `json:"bin_id"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	CanSort    bool    `json:"can_sort"`
	Reason     string  `json:"reason,omitempty"`
}

// ScanResult is what the user sees. It never contains the image URL.
type ScanResult struct {
	Category   string    `json:"category" firestore:"category"`
	Confidence float64   `json:"confidence" firestore:"confidence"`
	CanSort    bool      `json:"can_sort" firestore:"can_sort"`
	Reason     string    `json:"reason" firestore:"reason"`
	BinID      string    `json:"bin_id" firestore:"bin_id"`
	ScannedAt  time.Time `json:"scanned_at" firestore:"scanned_at"`
}
