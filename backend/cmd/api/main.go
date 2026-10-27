package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/handlers"
	"raspberry-bin-api/internal/middleware"
	"raspberry-bin-api/internal/repository"
	"raspberry-bin-api/internal/storage"

	"github.com/gin-gonic/gin"
	"golang.org/x/time/rate"
)

const (
	defaultBucketName = "raspberry-bin-67d4f.firebasestorage.app"

	// Image limit plus room for the other multipart fields
	maxScanBodySize = handlers.MaxImageSize + 1<<20

	// Minimum length for the device API key (32 random bytes in hex = 64 chars)
	minAPIKeyLength = 32

	// A login request is only a small JSON object
	maxLoginBodySize = 4 << 10
)

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

// parseTrustedProxies turns "a,b" into a list; empty means trust no proxy.
func parseTrustedProxies(value string) []string {
	var proxies []string
	for _, p := range strings.Split(value, ",") {
		if p = strings.TrimSpace(p); p != "" {
			proxies = append(proxies, p)
		}
	}
	return proxies
}

func main() {
	bucketName := getEnv("FIREBASE_BUCKET", defaultBucketName)
	credentialsFile := getEnv("FIREBASE_CREDENTIALS", "serviceAccountKey.json")
	aiServiceURL := getEnv("AI_SERVICE_URL", "http://localhost:5000")
	port := getEnv("PORT", "8080")

	deviceAPIKey := os.Getenv("DEVICE_API_KEY")
	if len(deviceAPIKey) < minAPIKeyLength {
		log.Fatalf("DEVICE_API_KEY must be set and at least %d characters long", minAPIKeyLength)
	}

	firebaseApp := config.InitFirebase(bucketName, credentialsFile)
	defer firebaseApp.DB.Close()

	scanRepo := repository.NewScanRepository(firebaseApp.DB)
	aiClient := clients.NewAIClient(aiServiceURL)
	fileStorage := storage.NewFileStorage(firebaseApp.Storage, bucketName)

	scanHandler := handlers.NewScanHandler(scanRepo, aiClient, fileStorage)

	authService := auth.NewService(repository.NewAdminRepository(firebaseApp.DB))
	adminHandler := handlers.NewAdminHandler(authService)

	r := gin.Default()
	r.MaxMultipartMemory = maxScanBodySize

	// Only take the client IP from X-Forwarded-For when the request comes
	// from our own reverse proxy; otherwise anyone could fake their IP.
	if err := r.SetTrustedProxies(parseTrustedProxies(os.Getenv("TRUSTED_PROXIES"))); err != nil {
		log.Fatalf("invalid TRUSTED_PROXIES: %v", err)
	}

	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// One scan every 2 seconds per IP, with short bursts of up to 5
	scanLimiter := middleware.NewRateLimiter(rate.Every(2*time.Second), 5)

	api := r.Group("/api", scanLimiter.Middleware(), middleware.APIKeyAuth(deviceAPIKey))
	{
		api.POST("/scan", middleware.MaxBodySize(maxScanBodySize), scanHandler.HandleScan)
	}

	// Admin routes are used by the web dashboard, so they use admin sessions
	// instead of the device API key. Login gets its own, stricter rate limit
	// on top of the 5-attempt account lock.
	loginLimiter := middleware.NewRateLimiter(rate.Every(3*time.Second), 5)
	r.POST("/api/admin/login", loginLimiter.Middleware(), middleware.MaxBodySize(maxLoginBodySize), adminHandler.Login)

	admin := r.Group("/api/admin", middleware.RequireAdmin(authService))
	{
		admin.GET("/me", adminHandler.Me)
		admin.POST("/logout", adminHandler.Logout)
	}

	r.Run(":" + port)
}
