// create-bin registers a bin, or gives it a new secret code if the printed QR
// was leaked (the old QR stops working). Only the code's hash is stored.
//
// On the server, show the QR in the terminal:
//
//	sudo docker compose run --rm api ./create-bin -bin bin-27
//
// or save it as a PNG for printing:
//
//	sudo docker compose run --rm -T api ./create-bin -bin bin-27 -png > bin-27.png
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"

	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/bins"
	"raspberry-bin-api/internal/repository"

	"github.com/skip2/go-qrcode"
)

const defaultBucketName = "raspberry-bin-67d4f.firebasestorage.app"

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func main() {
	binID := flag.String("bin", "", "bin ID, e.g. bin-27")
	asPNG := flag.Bool("png", false, "write the QR code as PNG to stdout instead of text")
	flag.Parse()
	if *binID == "" {
		log.Fatal("usage: create-bin -bin <bin-id> [-png]")
	}

	firebaseApp := config.InitFirebase(
		getEnv("FIREBASE_BUCKET", defaultBucketName),
		getEnv("FIREBASE_CREDENTIALS", "serviceAccountKey.json"),
	)
	defer firebaseApp.DB.Close()

	service := bins.NewService(repository.NewBinRepository(firebaseApp.DB))
	payload, err := service.SetNewCode(context.Background(), *binID)
	if err != nil {
		log.Fatalf("could not create bin: %v", err)
	}

	if *asPNG {
		png, err := qrcode.Encode(payload, qrcode.Medium, 512)
		if err != nil {
			log.Fatalf("could not create QR code: %v", err)
		}
		os.Stdout.Write(png)
		fmt.Fprintf(os.Stderr, "Bin %q has a new code; QR written as PNG.\n", *binID)
		return
	}

	qr, err := qrcode.New(payload, qrcode.Medium)
	if err != nil {
		log.Fatalf("could not create QR code: %v", err)
	}
	fmt.Println(qr.ToSmallString(false))
	fmt.Printf("QR content: %s\n\n", payload)
	fmt.Printf("Bin %q has a new code. Print this QR and stick it on the bin.\n", *binID)
	fmt.Println("Any older QR for this bin no longer works. Do not commit or share the code.")
}
