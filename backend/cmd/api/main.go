package main

import (
	"os"
	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/handlers"
	"raspberry-bin-api/internal/repository"

	"github.com/gin-gonic/gin"
)

func main() {
	if err := os.MkdirAll("uploads", 0755); err != nil {
		panic("Failed to create uploads directory")
	}

	db := config.InitFirestore()
	defer db.Close()

	scanRepo := repository.NewScanRepository(db)
	
	aiClient := clients.NewAIClient("http://localhost:5000")
	
	scanHandler := handlers.NewScanHandler(scanRepo, aiClient)

	r := gin.Default()

	r.Static("/uploads", "./uploads")

	api := r.Group("/api")
	{
		api.POST("/scan", scanHandler.HandleScan)
	}

	r.Run(":8080")
}