package models

import "mime/multipart"

type ScanRequest struct {
	BinID string                `form:"bin_id" binding:"required"`
	Image *multipart.FileHeader `form:"image" binding:"required"`
}

// ScanResponse tells the Pi what to do: open the lid for Category only if CanSort is true.
type ScanResponse struct {
	BinID      string  `json:"bin_id"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	CanSort    bool    `json:"can_sort"`
	Reason     string  `json:"reason,omitempty"`
}
