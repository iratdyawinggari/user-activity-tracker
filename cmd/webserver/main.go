package main

import (
	"log"
	"os"
	"time"

	"user-activity-tracker/configs"
	"user-activity-tracker/internal/cache"
	"user-activity-tracker/internal/database"
	"user-activity-tracker/internal/handlers"
	"user-activity-tracker/internal/middleware"
	"user-activity-tracker/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

// @title User Activity Tracker API
// @version 1.0
// @description A high-performance backend system for tracking user API usage
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.email support@activitytracker.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:8080
// @BasePath /api

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name X-API-Key

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Type "Bearer" followed by a space and JWT token.

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using default configuration")
	}

	// Load configuration
	if err := configs.LoadConfig(); err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// Initialize database
	database.GetDBManager()

	// Initialize cache and warm it up
	cacheMgr := cache.GetCacheManager()
	cacheMgr.WarmUsageCache()

	// Initialize services
	authService := services.NewAuthService()

	// Initialize handlers
	clientHandler := handlers.NewClientHandler(authService)
	wsHandler := handlers.NewWebSocketHandler()

	// Setup Gin router
	if os.Getenv("GIN_MODE") != "debug" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()

	// Global middleware
	router.Use(middleware.ValidationMiddleware())
	router.Use(gin.Recovery())

	// CORS middleware
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, X-API-Key")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	})

	// Public routes
	router.POST("/api/register", clientHandler.RegisterClient)

	// Protected routes
	protected := router.Group("/api")
	protected.Use(middleware.AuthMiddleware(authService))
	protected.Use(middleware.RateLimitMiddleware(cacheMgr))

	protected.POST("/logs", clientHandler.RecordLog)
	protected.GET("/usage/daily", clientHandler.GetDailyUsage)
	protected.GET("/usage/top", clientHandler.GetTopClients)

	// WebSocket route
	if configs.AppConfig.EnableWebSocket {
		router.GET("/ws", wsHandler.HandleConnections)
	}

	// Health check
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "healthy",
			"timestamp": time.Now().Unix(),
			"services": map[string]string{
				"database": "connected",
				"redis": func() string {
					if cacheMgr.IsAvailable() {
						return "connected"
					} else {
						return "local_cache_only"
					}
				}(),
				"cache": "active",
			},
		})
	})

	// Start server
	port := ":" + configs.AppConfig.ServerPort
	log.Printf("Server starting on port %s", port)
	log.Printf("Swagger docs available at http://localhost%s/swagger/index.html", port)

	if err := router.Run(port); err != nil {
		log.Fatal("Failed to start server:", err)
	}
}
