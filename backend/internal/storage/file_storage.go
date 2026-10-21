package storage

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"time"

	"firebase.google.com/go/v4/storage"
)

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

func (s *FileStorage) UploadImage(file *multipart.FileHeader) (string, error) {
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

	filename := fmt.Sprintf("%d-%s", time.Now().UnixNano(), file.Filename)
	obj := bucket.Object(filename)
	writer := obj.NewWriter(ctx)

	if _, err := io.Copy(writer, f); err != nil {
		return "", err
	}
	if err := writer.Close(); err != nil {
		return "", err
	}

	url := fmt.Sprintf("https://firebasestorage.googleapis.com/v0/b/%s/o/%s?alt=media", s.Bucket, filename)
	return url, nil
}