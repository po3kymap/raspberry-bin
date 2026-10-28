// Package bins checks the secret code printed as a QR code on each bin.
// Scanning the QR proves that the user is standing at the bin, so a scan
// (and the streak it earns) cannot be started from somewhere else.
package bins

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"
)

// QRPrefix starts every QR payload: "rbin:<bin_id>:<code>"
const QRPrefix = "rbin"

var (
	ErrBinNotFound    = errors.New("bin not found")
	ErrInvalidBinCode = errors.New("invalid bin code")
	ErrInvalidBinID   = errors.New("invalid bin_id")

	binIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,32}$`)
)

// Bin is stored in the "bins" collection, with the bin ID as document ID.
// Only the hash of the QR code is stored, like a password.
type Bin struct {
	CodeHash  string    `firestore:"code_hash"`
	CreatedAt time.Time `firestore:"created_at"`
}

type Store interface {
	GetBin(ctx context.Context, binID string) (*Bin, error)
	SaveBin(ctx context.Context, binID string, bin Bin) error
}

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func ValidateBinID(binID string) error {
	if !binIDPattern.MatchString(binID) {
		return ErrInvalidBinID
	}
	return nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// Exists reports whether the bin is registered.
func (s *Service) Exists(ctx context.Context, binID string) error {
	if err := ValidateBinID(binID); err != nil {
		return err
	}
	_, err := s.store.GetBin(ctx, binID)
	return err
}

// VerifyCode checks the code from the QR. Unknown bins and wrong codes give
// the same error, so the answer does not reveal which bin IDs exist.
func (s *Service) VerifyCode(ctx context.Context, binID, code string) error {
	if err := ValidateBinID(binID); err != nil {
		return ErrInvalidBinCode
	}

	bin, err := s.store.GetBin(ctx, binID)
	if errors.Is(err, ErrBinNotFound) {
		return ErrInvalidBinCode
	}
	if err != nil {
		return err
	}

	provided := hashCode(code)
	if subtle.ConstantTimeCompare([]byte(provided), []byte(bin.CodeHash)) != 1 {
		return ErrInvalidBinCode
	}
	return nil
}

// SetNewCode registers the bin (or replaces its code, e.g. when the printed
// QR was leaked) and returns the payload to put in the QR code.
func (s *Service) SetNewCode(ctx context.Context, binID string) (string, error) {
	if err := ValidateBinID(binID); err != nil {
		return "", err
	}

	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(b)

	err := s.store.SaveBin(ctx, binID, Bin{CodeHash: hashCode(code), CreatedAt: time.Now()})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s:%s:%s", QRPrefix, binID, code), nil
}
