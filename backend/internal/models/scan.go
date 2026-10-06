package models

type ScanRequest struct {
	BinID string `json:"bin_id" binding:"required"`
}

type ScanResponse struct {
	BinID    string  `json:"bin_id"`
	Category string  `json:"category"`
	Confidence float64 `json:"confidence"`
}