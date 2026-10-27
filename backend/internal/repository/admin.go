package repository

import (
	"context"
	"time"

	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/models"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	adminsCollection   = "admins"
	sessionsCollection = "admin_sessions"
)

// AdminRepository stores admins and their sessions in Firestore.
// It implements auth.Store.
type AdminRepository struct {
	DB *firestore.Client
}

func NewAdminRepository(db *firestore.Client) *AdminRepository {
	return &AdminRepository{DB: db}
}

func (r *AdminRepository) GetAdmin(ctx context.Context, username string) (*models.Admin, error) {
	doc, err := r.DB.Collection(adminsCollection).Doc(username).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, auth.ErrAdminNotFound
	}
	if err != nil {
		return nil, err
	}

	var admin models.Admin
	if err := doc.DataTo(&admin); err != nil {
		return nil, err
	}
	return &admin, nil
}

func (r *AdminRepository) CreateAdmin(ctx context.Context, admin models.Admin) error {
	// Create fails if the document already exists, so an admin is never overwritten
	_, err := r.DB.Collection(adminsCollection).Doc(admin.Username).Create(ctx, admin)
	if status.Code(err) == codes.AlreadyExists {
		return auth.ErrAdminExists
	}
	return err
}

func (r *AdminRepository) UpdateLoginState(ctx context.Context, username string, failedAttempts int, lockedUntil time.Time) error {
	_, err := r.DB.Collection(adminsCollection).Doc(username).Update(ctx, []firestore.Update{
		{Path: "failed_attempts", Value: failedAttempts},
		{Path: "locked_until", Value: lockedUntil},
	})
	return err
}

func (r *AdminRepository) CreateSession(ctx context.Context, tokenHash string, session models.AdminSession) error {
	_, err := r.DB.Collection(sessionsCollection).Doc(tokenHash).Create(ctx, session)
	return err
}

func (r *AdminRepository) GetSession(ctx context.Context, tokenHash string) (*models.AdminSession, error) {
	doc, err := r.DB.Collection(sessionsCollection).Doc(tokenHash).Get(ctx)
	if status.Code(err) == codes.NotFound {
		return nil, auth.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}

	var session models.AdminSession
	if err := doc.DataTo(&session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *AdminRepository) DeleteSession(ctx context.Context, tokenHash string) error {
	_, err := r.DB.Collection(sessionsCollection).Doc(tokenHash).Delete(ctx)
	return err
}
