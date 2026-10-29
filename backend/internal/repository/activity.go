package repository

import (
	"context"
	"time"

	"raspberry-bin-api/internal/activity"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
)

const activityCollection = "activity"

// ActivityRepository implements activity.Store.
type ActivityRepository struct {
	DB *firestore.Client
}

func NewActivityRepository(db *firestore.Client) *ActivityRepository {
	return &ActivityRepository{DB: db}
}

func (r *ActivityRepository) AddEvent(ctx context.Context, event activity.Event) error {
	_, _, err := r.DB.Collection(activityCollection).Add(ctx, event)
	return err
}

// ListEvents returns the newest events first. If before is set, only events
// older than it are returned (for loading the next page).
func (r *ActivityRepository) ListEvents(ctx context.Context, limit int, before time.Time) ([]activity.Event, error) {
	query := r.DB.Collection(activityCollection).OrderBy("at", firestore.Desc)
	if !before.IsZero() {
		query = query.Where("at", "<", before)
	}
	iter := query.Limit(limit).Documents(ctx)
	defer iter.Stop()

	events := []activity.Event{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return events, nil
		}
		if err != nil {
			return nil, err
		}
		var event activity.Event
		if err := doc.DataTo(&event); err != nil {
			return nil, err
		}
		event.ID = doc.Ref.ID
		events = append(events, event)
	}
}
