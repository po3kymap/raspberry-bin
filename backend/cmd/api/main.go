package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	_ "time/tzdata" // time zone data built in, so Europe/Amsterdam works in any container

	"raspberry-bin-api/config"
	"raspberry-bin-api/internal/activity"
	"raspberry-bin-api/internal/auth"
	"raspberry-bin-api/internal/bins"
	"raspberry-bin-api/internal/clients"
	"raspberry-bin-api/internal/dispatch"
	"raspberry-bin-api/internal/handlers"
	"raspberry-bin-api/internal/middleware"
	"raspberry-bin-api/internal/models"
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

	// Login, register and other JSON requests are small objects
	maxJSONBodySize = 4 << 10

	// Open long-polling requests allowed per IP
	maxOpenPollsPerIP = 20
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

	// Days for streaks and statistics are counted in the bin's time zone
	location, err := time.LoadLocation(getEnv("TIME_ZONE", "Europe/Amsterdam"))
	if err != nil {
		log.Fatalf("invalid TIME_ZONE: %v", err)
	}

	firebaseApp := config.InitFirebase(bucketName, credentialsFile)
	defer firebaseApp.DB.Close()

	userRepo := repository.NewUserRepository(firebaseApp.DB)
	scanRepo := repository.NewScanRepository(firebaseApp.DB, location)
	authService := auth.NewService(userRepo)
	binService := bins.NewService(repository.NewBinRepository(firebaseApp.DB))
	dispatcher := dispatch.New(dispatch.DefaultConfig())
	activityRepo := repository.NewActivityRepository(firebaseApp.DB)
	recorder := activity.NewRecorder(activityRepo)

	fileStorage := storage.NewFileStorage(firebaseApp.Storage, bucketName)
	scanHandler := handlers.NewScanHandler(scanRepo, clients.NewAIClient(aiServiceURL), fileStorage, dispatcher, recorder)
	authHandler := handlers.NewAuthHandler(authService, recorder)
	scanRequestHandler := handlers.NewScanRequestHandler(dispatcher, binService)
	deviceHandler := handlers.NewDeviceHandler(dispatcher, binService, recorder)
	meHandler := handlers.NewMeHandler(scanRepo, userRepo, location)
	adminHandler := handlers.NewAdminHandler(authService, userRepo, scanRepo, binService, recorder, activityRepo, fileStorage, location)

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

	// Rate limits. Per IP for anonymous routes; per user for logged-in routes,
	// so many students behind one university IP do not block each other.
	loginLimiter := middleware.NewRateLimiter(rate.Every(3*time.Second), 5)
	registerLimiter := middleware.NewRateLimiter(rate.Every(10*time.Second), 3)
	scanStartLimiter := middleware.NewRateLimiter(rate.Every(3*time.Second), 3)
	userLimiter := middleware.NewRateLimiter(rate.Every(200*time.Millisecond), 20)
	deviceLimiter := middleware.NewRateLimiter(rate.Every(500*time.Millisecond), 10)
	perUser := func(c *gin.Context) string { return c.GetString(middleware.UsernameKey) }
	jsonBody := middleware.MaxBodySize(maxJSONBodySize)
	longPoll := middleware.MaxConcurrentPerIP(maxOpenPollsPerIP)

	// Accounts
	authRoutes := r.Group("/api/auth")
	{
		authRoutes.POST("/register", registerLimiter.Middleware(), jsonBody, authHandler.Register)
		authRoutes.POST("/login", loginLimiter.Middleware(), jsonBody, authHandler.Login)

		session := authRoutes.Group("", middleware.RequireSession(authService))
		session.POST("/logout", authHandler.Logout)
		session.GET("/me", authHandler.Me)
	}

	// Logged-in users: start a scan (QR code) and read their own data
	user := r.Group("/api", middleware.RequireSession(authService))
	{
		user.POST("/scan-requests", scanStartLimiter.MiddlewareByKey(perUser), jsonBody, scanRequestHandler.Create)
		user.GET("/scan-requests/:id", userLimiter.MiddlewareByKey(perUser), longPoll, scanRequestHandler.Get)
		user.GET("/me/scans", userLimiter.MiddlewareByKey(perUser), meHandler.Scans)
		user.GET("/me/stats", userLimiter.MiddlewareByKey(perUser), meHandler.Stats)
	}

	// Admins. The role is checked on every request, so a user token never opens these.
	r.POST("/api/admin/login", loginLimiter.Middleware(), jsonBody, authHandler.AdminLogin)
	admin := r.Group("/api/admin", middleware.RequireSession(authService), middleware.RequireRole(models.RoleAdmin))
	{
		admin.GET("/me", authHandler.Me)
		admin.POST("/logout", authHandler.Logout)
		admin.POST("/lids/:compartment/:action", jsonBody, deviceHandler.Lid)

		admin.GET("/users", userLimiter.MiddlewareByKey(perUser), adminHandler.ListUsers)
		admin.POST("/users", userLimiter.MiddlewareByKey(perUser), jsonBody, adminHandler.CreateUser)
		admin.DELETE("/users/:username", userLimiter.MiddlewareByKey(perUser), adminHandler.DeleteUser)
		admin.GET("/stats", userLimiter.MiddlewareByKey(perUser), adminHandler.Stats)
		admin.GET("/activity", userLimiter.MiddlewareByKey(perUser), adminHandler.ListActivity)
		admin.GET("/images/:file", userLimiter.MiddlewareByKey(perUser), adminHandler.ScanImage)
		admin.POST("/bins/connect", scanStartLimiter.MiddlewareByKey(perUser), jsonBody, adminHandler.ConnectBin)
	}

	// The bin (Raspberry Pi), authenticated with the device API key
	device := r.Group("/api", deviceLimiter.Middleware(), middleware.APIKeyAuth(deviceAPIKey))
	{
		device.GET("/device/commands", longPoll, deviceHandler.Commands)
		device.POST("/scan", middleware.MaxBodySize(maxScanBodySize), scanHandler.HandleScan)
	}

	// Timeouts stop slow clients from holding connections open forever.
	// WriteTimeout must be longer than the longest long-polling wait.
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      90 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}
