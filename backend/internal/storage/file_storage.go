package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"
	"regexp"

	gcs "cloud.google.com/go/storage"
	"firebase.google.com/go/v4/storage"
	"github.com/google/uuid"
)

// Names UploadImage creates: "scans/<uuid>.jpg" or ".png"
var imageNamePattern = regexp.MustCompile(`^scans/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}\.(jpg|png)$`)

var (
	ErrInvalidImageName = errors.New("invalid image name")
	ErrImageNotFound    = errors.New("image not found")
)

var extensions = map[string]string{
	"image/jpeg": ".jpg",
	"image/png":  ".png",
}

type FileStorage struct {
	Client *storage.Client
	Bucket string
}

func NewFileStorage(client *storage.Client, bucket string) *FileStorage {
	return &FileStorage{
		Client: client,
		Bucket: bucket,
	}
}

// UploadImage stores the file under a random name and returns its URL and the
// object name (e.g. "scans/<uuid>.jpg"). The client's file name is never used,
// so it cannot influence the storage path.
func (s *FileStorage) UploadImage(file *multipart.FileHeader, contentType string) (imageURL, objectName string, err error) {
	ext, ok := extensions[contentType]
	if !ok {
		return "", "", fmt.Errorf("unsupported content type %q", contentType)
	}

	ctx := context.Background()
	bucket, err := s.Client.Bucket(s.Bucket)
	if err != nil {
		return "", "", err
	}

	f, err := file.Open()
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	filename := "scans/" + uuid.NewString() + ext
	obj := bucket.Object(filename)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, f); err != nil {
		writer.Close()
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}

	// The object name must be escaped as one path segment ("/" becomes "%2F")
	imageURL = fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media", s.Bucket, url.PathEscape(filename))
	return imageURL, filename, nil
}

// OpenImage returns a scan photo by the name UploadImage gave it. Only such
// names are accepted, so no other file in the bucket can be read this way.
// The caller must close the reader.
func (s *FileStorage) OpenImage(ctx context.Context, name string) (io.ReadCloser, string, error) {
	if !imageNamePattern.MatchString(name) {
		return nil, "", ErrInvalidImageName
	}

	bucket, err := s.Client.Bucket(s.Bucket)
	if err != nil {
		return nil, "", err
	}
	reader, err := bucket.Object(name).NewReader(ctx)
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return nil, "", ErrImageNotFound
	}
	if err != nil {
		return nil, "", err
	}
	return reader, reader.Attrs.ContentType, nil
}
