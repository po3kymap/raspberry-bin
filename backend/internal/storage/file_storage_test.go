package storage

import (
	"context"
	"errors"
	"testing"
)

func TestOpenImageOnlyAcceptsScanPhotoNames(t *testing.T) {
	s := &FileStorage{}

	// Every one of these is refused before the bucket is touched
	for _, name := range []string{
		"",
		"scans/../serviceAccountKey.json",
		"scans/5f1c2d3e-0000-4000-8000-123456789abc.gif",
		"other/5f1c2d3e-0000-4000-8000-123456789abc.jpg",
		"scans/5F1C2D3E-0000-4000-8000-123456789ABC.jpg",
		"scans/5f1c2d3e-0000-4000-8000-123456789abc.jpg/extra",
	} {
		if _, _, err := s.OpenImage(context.Background(), name); !errors.Is(err, ErrInvalidImageName) {
			t.Errorf("OpenImage(%q) error = %v, want %v", name, err, ErrInvalidImageName)
		}
	}
}

func TestImageNamePatternMatchesUploadedNames(t *testing.T) {
	for _, name := range []string{
		"scans/5f1c2d3e-0000-4000-8000-123456789abc.jpg",
		"scans/062f772e-b880-42cd-a2b2-8e1d62714435.png",
	} {
		if !imageNamePattern.MatchString(name) {
			t.Errorf("%q should be accepted", name)
		}
	}
}
