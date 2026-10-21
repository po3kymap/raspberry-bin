package models

import "mime/multipart"

type ScanRequest struct {
	BinID string                `form:"bin_id" binding:"required"`
	Image *multipart.FileHeader `form:"image" binding:"required"`
}

type ScanResponse struct {
	BinID     string  `json:"bin_id"`
	Category  string  `json:"category"`
	Confidence float64 `json:"confidence"`
}