// create-admin adds an admin account. There is no API endpoint for this on
// purpose: only someone with access to the server can create admins.
//
// On the server:
//
//	sudo docker compose run --rm api ./create-admin -username <name>
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/repository"

	"golang.org/x/term"
)

const defaultBucketName = "raspberry-bin-67d4f.firebasestorage.app"

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// readPassword reads a line from the terminal without showing what is typed.
func readPassword(prompt string) string {
	fmt.Print(prompt)
	password, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Println()
	if err != nil {
		log.Fatalf("could not read password: %v", err)
	}
	return string(password)
}

func main() {
	username := flag.String("username", "", "admin username")
	flag.Parse()
	if *username == "" {
		log.Fatal("usage: create-admin -username <name>")
	}

	password := readPassword("Password: ")
	if readPassword("Repeat password: ") != password {
		log.Fatal("passwords do not match")
	}

	firebaseApp := config.InitFirebase(
		getEnv("FIREBASE_BUCKET", defaultBucketName),
		getEnv("FIREBASE_CREDENTIALS", "serviceAccountKey.json"),
	)
	defer firebaseApp.DB.Close()

	authService := auth.NewService(repository.NewAdminRepository(firebaseApp.DB))
	if err := authService.CreateAdmin(context.Background(), *username, password); err != nil {
		log.Fatalf("could not create admin: %v", err)
	}

	fmt.Printf("Admin %q created\n", *username)
}
