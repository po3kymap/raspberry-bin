package config

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/storage"
	"google.golang.org/api/option"
)

type FirebaseApp struct {
	DB      *firestore.Client
	Storage *storage.Client
}

func InitFirebase(bucketName, credentialsFile string) *FirebaseApp {
	opt := option.WithCredentialsFile(credentialsFile)
	config := &firebase.Config{
		StorageBucket: bucketName,
	}

	app, err := firebase.NewApp(context.Background(), config, opt)
	if err != nil {
		log.Fatalf("Firebase init error: %v\n", err)
	}

	db, err := app.Firestore(context.Background())
	if err != nil {
		log.Fatalf("Firestore connect error: %v\n", err)
	}

	storageClient, err := app.Storage(context.Background())
	if err != nil {
		log.Fatalf("Storage connect error: %v\n", err)
	}

	return &FirebaseApp{
		DB:      db,
		Storage: storageClient,
	}
}