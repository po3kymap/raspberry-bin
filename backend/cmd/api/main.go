package main

import (
	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/handlers"

	"github.com/gin-gonic/gin"
)

func main() {
	db := config.InitFirestore()
	defer db.Close()

	scanHandler := handlers.NewScanHandler(db)

	r := gin.Default()

	api := r.Group("/api")
	{
		api.POST("/scan", scanHandler.HandleScan)
	}

	r.Run(":8080")
}
