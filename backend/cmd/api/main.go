package main

import (
	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/handlers"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
)

const bucketName = "raspberry-bin-67d4f.firebasestorage.app"

func main() {
	firebaseApp := config.InitFirebase(bucketName)
	defer firebaseApp.DB.Close()

	scanRepo := repository.NewScanRepository(firebaseApp.DB)
	aiClient := clients.NewAIClient("http://localhost:5000")
	fileStorage := storage.NewFileStorage(firebaseApp.Storage, bucketName)

	scanHandler := handlers.NewScanHandler(scanRepo, aiClient, fileStorage)

	r := gin.Default()

	api := r.Group("/api")
	{
		api.POST("/scan", scanHandler.HandleScan)
	}

	r.Run(":8080")
}