package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/url"

	"firebase.google.com/go/v4/storage"
	"github.com/google/uuid"
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

// UploadImage stores the file under a random name. The client's file name
// is never used, so it cannot influence the storage path.
func (s *FileStorage) UploadImage(file *multipart.FileHeader, contentType string) (string, error) {
	ext, ok := extensions[contentType]
	if !ok {
		return "", fmt.Errorf("unsupported content type %q", contentType)
	}

	ctx := context.Background()
	bucket, err := s.Client.Bucket(s.Bucket)
	if err != nil {
		return "", err
	}

	f, err := file.Open()
	if err != nil {
		return "", err
	}
	defer f.Close()

	filename := "scans/" + uuid.NewString() + ext
	obj := bucket.Object(filename)
	writer := obj.NewWriter(ctx)
	writer.ContentType = contentType

	if _, err := io.Copy(writer, f); err != nil {
		writer.Close()
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	// The object name must be escaped as one path segment ("/" becomes "%2F")
	imageURL := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media", s.Bucket, url.PathEscape(filename))
	return imageURL, nil
}
