package config

import (
	"context"
	"log"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"google.golang.org/api/option"
)

func InitFirestore() *firestore.Client {
	opt := option.WithCredentialsFile("serviceAccountKey.json")

	app, err := firebase.NewApp(context.Background(), nil, opt)
	if err != nil {
		log.Fatalf("Firebase init error: %v\n", err)
	}

	client, err := app.Firestore(context.Background())
	if err != nil {
		log.Fatalf("Firestore connect error: %v\n", err)
	}

	return client
}