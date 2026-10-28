package repository

import (
	"context"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/models"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	usersCollection    = "users"
	sessionsCollection = "sessions"
)

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
