package repository

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
)

const checksCollection = "checks"

// Check is a photo a user took at home to see which bin an item belongs in.
// The photos are kept as extra training data for the model; they never count
// for streaks, statistics or the activity log.
type Check struct {
	Username   string    `firestore:"username"`
	ImageURL   string    `firestore:"image_url"`
	Category   string    `firestore:"category"`
	Confidence float64   `firestore:"confidence"`
	CanSort    bool      `firestore:"can_sort"`
	Reason     string    `firestore:"reason"`
	CheckedAt  time.Time `firestore:"checked_at"`
}

type CheckRepository struct {
	DB *firestore.Client
}

func NewCheckRepository(db *firestore.Client) *CheckRepository {
	return &CheckRepository{DB: db}
}

func (r *CheckRepository) SaveCheck(ctx context.Context, check Check) error {
	_, _, err := r.DB.Collection(checksCollection).Add(ctx, check)
	return err
}
