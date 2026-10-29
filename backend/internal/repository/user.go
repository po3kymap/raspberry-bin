package repository

import (
	"context"
	"errors"
	"time"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/models"

	"cloud.google.com/go/firestore"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	usersCollection    = "users"
	sessionsCollection = "sessions"

	// Upper limit for the admin user list; far more than this project will have
	maxListedUsers = 1000
)

var ErrLastAdmin = errors.New("the last admin cannot be deleted")

// UserRepository stores users and their sessions in Firestore.
// It implements auth.Store.
type UserRepository struct {
	DB *firestore.Client
}

func NewUserRepository(db *firestore.Client) *UserRepository {
	return &UserRepository{DB: db}
}

func (r *UserRepository) GetUser(ctx context.Context, username string) (*models.User, error) {
	doc, err := r.DB.Collection(usersCollection).Doc(username).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, auth.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}

	var user models.User
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) CreateUser(ctx context.Context, user models.User) error {
	// Create fails if the document already exists, so an account is never overwritten
	_, err := r.DB.Collection(usersCollection).Doc(user.Username).Create(ctx, user)
	if status.Code(err) == codes.AlreadyExists {
		return auth.ErrUserExists
	}
	return err
}

func (r *UserRepository) UpdateUser(ctx context.Context, username string, fn func(*models.User) error) error {
	ref := r.DB.Collection(usersCollection).Doc(username)

	return r.DB.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(ref)
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
		if err := fn(&user); err != nil {
			return err
		}
		return tx.Set(ref, user)
	})
}

func (r *UserRepository) CreateSession(ctx context.Context, tokenHash string, session models.Session) error {
	_, err := r.DB.Collection(sessionsCollection).Doc(tokenHash).Create(ctx, session)
	return err
}

func (r *UserRepository) GetSession(ctx context.Context, tokenHash string) (*models.Session, error) {
	doc, err := r.DB.Collection(sessionsCollection).Doc(tokenHash).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, auth.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	var session models.Session
	if err := doc.DataTo(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *UserRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.DB.Collection(sessionsCollection).Doc(tokenHash).Delete(ctx)
	return err
}

func (r *UserRepository) SetLastLogin(ctx context.Context, username string, at time.Time) error {
	_, err := r.DB.Collection(usersCollection).Doc(username).Update(ctx, []firestore.Update{
		{Path: "last_login_at", Value: at},
	})
	return err
}

// ListUsers returns all users sorted by username. The password hash is not
// even read from the database.
func (r *UserRepository) ListUsers(ctx context.Context) ([]models.User, error) {
	iter := r.DB.Collection(usersCollection).
		Select("username", "role", "created_at", "last_login_at", "locked_until", "current_streak", "longest_streak", "last_sorted_date").
		OrderBy("username", firestore.Asc).
		Limit(maxListedUsers).
		Documents(ctx)
	defer iter.Stop()

	users := []models.User{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return users, nil
		}
		if err != nil {
			return nil, err
		}
		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
}

type UserCounts struct {
	Total  int `json:"total"`
	Admins int `json:"admins"`
	// Accounts created in the requested period
	New int `json:"new"`
}

// CountUsers counts all users, the admins among them and the accounts created in [from, to).
func (r *UserRepository) CountUsers(ctx context.Context, from, to time.Time) (UserCounts, error) {
	iter := r.DB.Collection(usersCollection).Select("role", "created_at").Documents(ctx)
	defer iter.Stop()

	var counts UserCounts
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			return counts, nil
		}
		if err != nil {
			return UserCounts{}, err
		}
		var user models.User
		if err := doc.DataTo(&user); err != nil {
			return UserCounts{}, err
		}
		counts.Total++
		if user.Role == models.RoleAdmin {
			counts.Admins++
		}
		if !user.CreatedAt.Before(from) && user.CreatedAt.Before(to) {
			counts.New++
		}
	}
}

// DeleteUser removes the account and all of its sessions in one transaction,
// so the user is logged out at once. The last admin cannot be deleted, so the
// admin screens can never lock everyone out. The user's scans stay in the
// global "scans" collection, so the overall statistics do not change.
func (r *UserRepository) DeleteUser(ctx context.Context, username string) error {
	userRef := r.DB.Collection(usersCollection).Doc(username)

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

		if user.Role == models.RoleAdmin {
			// Reading the admins inside the transaction means two admins
			// deleting each other at the same time cannot both succeed
			admins, err := tx.Documents(r.DB.Collection(usersCollection).
				Where("role", "==", models.RoleAdmin).Limit(2)).GetAll()
			if err != nil {
				return err
			}
			if len(admins) < 2 {
				return ErrLastAdmin
			}
		}

		sessions, err := tx.Documents(r.DB.Collection(sessionsCollection).
			Where("username", "==", username)).GetAll()
		if err != nil {
			return err
		}
		for _, session := range sessions {
			if err := tx.Delete(session.Ref); err != nil {
				return err
			}
		}
		return tx.Delete(userRef)
	})
	if err != nil {
		return err
	}

	// The account is gone; now remove the user's own copy of the history
	return r.deleteCollection(ctx, userRef.Collection(userScansCollection))
}

func (r *UserRepository) deleteCollection(ctx context.Context, collection *firestore.CollectionRef) error {
	writer := r.DB.BulkWriter(ctx)
	var jobs []*firestore.BulkWriterJob

	refs := collection.DocumentRefs(ctx)
	for {
		ref, err := refs.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			writer.End()
			return err
		}
		job, err := writer.Delete(ref)
		if err != nil {
			writer.End()
			return err
		}
		jobs = append(jobs, job)
	}

	// End waits until every delete has been sent
	writer.End()
	for _, job := range jobs {
		if _, err := job.Results(); err != nil {
			return err
		}
	}
	return nil
}
