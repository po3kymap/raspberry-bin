package repository

import (
	"context"

	"raspberry-bin-api/internal/bins"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const binsCollection = "bins"

// BinRepository implements bins.Store.
type BinRepository struct {
	DB *firestore.Client
}

func NewBinRepository(db *firestore.Client) *BinRepository {
	return &BinRepository{DB: db}
}

func (r *BinRepository) GetBin(ctx context.Context, binID string) (*bins.Bin, error) {
	doc, err := r.DB.Collection(binsCollection).Doc(binID).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, bins.ErrBinNotFound
	}
	if err != nil {
		return nil, err
	}

	var bin bins.Bin
	if err := doc.DataTo(&bin); err != nil {
		return nil, err
	}
	return &bin, nil
}

func (r *BinRepository) SaveBin(ctx context.Context, binID string, bin bins.Bin) error {
	_, err := r.DB.Collection(binsCollection).Doc(binID).Set(ctx, bin)
	return err
}
