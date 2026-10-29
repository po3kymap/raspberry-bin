package repository

import (
	"context"
	"time"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/models"
	"raspberry-bin-api/internal/stats"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	scansCollection = "scans"
	// users/{username}/scans: the user's own history, without image URLs
	userScansCollection = "scans"
)

// StoredScan is a scan as saved in the global "scans" collection.
// Only this collection holds the image URL; it is never sent to users.
type StoredScan struct {
	models.ScanResult
	Username  string `firestore:"username"`
	RequestID string `firestore:"request_id"`
	ImageURL  string `firestore:"image_url"`
}

type ScanRepository struct {
	DB       *firestore.Client
	Location *time.Location
}

func NewScanRepository(db *firestore.Client, loc *time.Location) *ScanRepository {
	return &ScanRepository{DB: db, Location: loc}
}

// SaveScan writes the scan to the global collection and to the user's
// history, and updates the user's streak, all in one transaction. It returns
// the ID of the new scan.
func (r *ScanRepository) SaveScan(ctx context.Context, scan StoredScan) (string, error) {
	userRef := r.DB.Collection(usersCollection).Doc(scan.Username)
	scanRef := r.DB.Collection(scansCollection).NewDoc()
	userScanRef := userRef.Collection(userScansCollection).Doc(scanRef.ID)

	err := r.DB.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		// Firestore transactions must read before they write
		doc, err := tx.Get(userRef)
		if status.Code(err) == codes.NotFound {
			return auth.ErrUserNotFound
		}
		if err != nil {
			return err
		}
		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return err
		}

		if err := tx.Create(scanRef, scan); err != nil {
			return err
		}
		if err := tx.Create(userScanRef, scan.ScanResult); err != nil {
			return err
		}

		// Only items that could be sorted count for the streak
		if !scan.CanSort {
			return nil
		}
		stats.UpdateStreak(&user, scan.ScannedAt.In(r.Location).Format(stats.DateLayout))
		return tx.Update(userRef, []firestore.Update{
			{Path: "current_streak", Value: user.CurrentStreak},
			{Path: "longest_streak", Value: user.LongestStreak},
			{Path: "last_sorted_date", Value: user.LastSortedDate},
		})
	})
	if err != nil {
		return "", err
	}
	return scanRef.ID, nil
}

// ListUserScans returns the user's scans, newest first. If before is set,
// only scans older than it are returned (for loading the next page).
func (r *ScanRepository) ListUserScans(ctx context.Context, username string, limit int, before time.Time) ([]models.ScanResult, error) {
	query := r.DB.Collection(usersCollection).Doc(username).Collection(userScansCollection).
		OrderBy("scanned_at", firestore.Desc)
	if !before.IsZero() {
		query = query.Where("scanned_at", "<", before)
	}
	return readScans(query.Limit(limit).Documents(ctx))
}

// UserScansBetween returns the user's scans in [from, to).
func (r *ScanRepository) UserScansBetween(ctx context.Context, username string, from, to time.Time) ([]models.ScanResult, error) {
	query := r.DB.Collection(usersCollection).Doc(username).Collection(userScansCollection).
		Where("scanned_at", ">=", from).
		Where("scanned_at", "<", to)
	return readScans(query.Documents(ctx))
}

// ScansBetween returns the scans in [from, to) for the admin statistics; with
// a binID only the scans of that bin. The image URL is not read.
func (r *ScanRepository) ScansBetween(ctx context.Context, binID string, from, to time.Time) ([]stats.UserScan, error) {
	iter := r.DB.Collection(scansCollection).
		Select("category", "confidence", "can_sort", "reason", "bin_id", "scanned_at", "username").
		Where("scanned_at", ">=", from).
		Where("scanned_at", "<", to).
		Documents(ctx)
	defer iter.Stop()

	scans := []stats.UserScan{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return scans, nil
		}
		if err != nil {
			return nil, err
		}
		var scan StoredScan
		if err := doc.DataTo(&scan); err != nil {
			return nil, err
		}
		// Filtered here instead of in the query: bin_id together with the
		// scanned_at range would need a composite index in Firestore
		if binID != "" && scan.BinID != binID {
			continue
		}
		scans = append(scans, stats.UserScan{Scan: scan.ScanResult, Username: scan.Username})
	}
}

// ScanCounts is how many scans a user made and how many of them were sorted.
type ScanCounts struct {
	Total  int
	Sorted int
}

// CountScansByUser counts every user's scans over all time, for the admin user list.
func (r *ScanRepository) CountScansByUser(ctx context.Context) (map[string]ScanCounts, error) {
	iter := r.DB.Collection(scansCollection).Select("username", "can_sort").Documents(ctx)
	defer iter.Stop()

	counts := map[string]ScanCounts{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return counts, nil
		}
		if err != nil {
			return nil, err
		}
		var scan StoredScan
		if err := doc.DataTo(&scan); err != nil {
			return nil, err
		}
		c := counts[scan.Username]
		c.Total++
		if scan.CanSort {
			c.Sorted++
		}
		counts[scan.Username] = c
	}
}

func readScans(iter *firestore.DocumentIterator) ([]models.ScanResult, error) {
	defer iter.Stop()

	scans := []models.ScanResult{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return scans, nil
		}
		if err != nil {
			return nil, err
		}
		var scan models.ScanResult
		if err := doc.DataTo(&scan); err != nil {
			return nil, err
		}
		scans = append(scans, scan)
	}
}
