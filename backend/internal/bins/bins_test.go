package bins

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type memoryStore map[string]Bin

func (s memoryStore) GetBin(_ context.Context, binID string) (*Bin, error) {
	bin, ok := s[binID]
	if !ok {
		return nil, ErrBinNotFound
	}
	return &bin, nil
}

func (s memoryStore) SaveBin(_ context.Context, binID string, bin Bin) error {
	s[binID] = bin
	return nil
}

func codeFromPayload(t *testing.T, payload string) string {
	t.Helper()
	parts := strings.Split(payload, ":")
	if len(parts) != 3 || parts[0] != QRPrefix {
		t.Fatalf("payload %q is not rbin:<bin>:<code>", payload)
	}
	return parts[2]
}

func TestQRCodeIsVerifiedAndOnlyItsHashIsStored(t *testing.T) {
	store := memoryStore{}
	service := NewService(store)
	ctx := context.Background()

	payload, err := service.SetNewCode(ctx, "bin-27")
	if err != nil {
		t.Fatal(err)
	}
	code := codeFromPayload(t, payload)

	if strings.Contains(store["bin-27"].CodeHash, code) {
		t.Error("the plain code is stored")
	}
	if err := service.VerifyCode(ctx, "bin-27", code); err != nil {
		t.Errorf("correct code rejected: %v", err)
	}
}

func TestWrongCodeAndUnknownBinGiveSameError(t *testing.T) {
	service := NewService(memoryStore{})
	ctx := context.Background()
	payload, _ := service.SetNewCode(ctx, "bin-27")
	code := codeFromPayload(t, payload)

	tests := []struct{ name, bin, code string }{
		{"wrong code", "bin-27", "made-up-code"},
		{"empty code", "bin-27", ""},
		{"right code, other bin", "bin-99", code},
		{"invalid bin ID", "../bin", code},
	}
	for _, tt := range tests {
		if err := service.VerifyCode(ctx, tt.bin, tt.code); !errors.Is(err, ErrInvalidBinCode) {
			t.Errorf("%s: error = %v, want %v", tt.name, err, ErrInvalidBinCode)
		}
	}
}

// A leaked QR stops working when the admin gives the bin a new code.
func TestNewCodeReplacesOldOne(t *testing.T) {
	service := NewService(memoryStore{})
	ctx := context.Background()

	oldPayload, _ := service.SetNewCode(ctx, "bin-27")
	newPayload, _ := service.SetNewCode(ctx, "bin-27")

	if err := service.VerifyCode(ctx, "bin-27", codeFromPayload(t, oldPayload)); !errors.Is(err, ErrInvalidBinCode) {
		t.Errorf("old code after rotation: error = %v, want %v", err, ErrInvalidBinCode)
	}
	if err := service.VerifyCode(ctx, "bin-27", codeFromPayload(t, newPayload)); err != nil {
		t.Errorf("new code rejected: %v", err)
	}
}
