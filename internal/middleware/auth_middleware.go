package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"user-activity-tracker/configs"
	"user-activity-tracker/internal/cache"
	"user-activity-tracker/internal/services"

	"github.com/gin-gonic/gin"
)

func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get API key from header or query parameter
		apiKey := c.GetHeader("X-API-Key")
		if apiKey == "" {
			apiKey = c.Query("api_key")
		}

		// Get JWT token
		authHeader := c.GetHeader("Authorization")
		var tokenString string
		if strings.HasPrefix(authHeader, "Bearer ") {
			tokenString = strings.TrimPrefix(authHeader, "Bearer ")
		}

		// Validate either API key or JWT token
		var clientID string
		if apiKey != "" {
			client, err := authService.ValidateAPIKey(apiKey)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid API key"})
				c.Abort()
				return
			}

			// Check IP whitelist if enabled
			if configs.AppConfig.EnableIPWhitelist {
				clientIP := c.ClientIP()
				if !authService.CheckIPWhitelist(client, clientIP) {
					c.JSON(http.StatusForbidden, gin.H{"error": "IP not whitelisted"})
					c.Abort()
					return
				}
			}

			clientID = client.ClientID
		} else if tokenString != "" {
			claims, err := authService.ValidateToken(tokenString)
			if err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
				c.Abort()
				return
			}
			clientID = claims.ClientID
		} else {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}

		// Store client info in context
		c.Set("client_id", clientID)
		c.Set("api_key", apiKey)

		c.Next()
	}
}

func RateLimitMiddleware(cache *cache.CacheManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		clientID, exists := c.Get("client_id")
		if !exists {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Client not authenticated"})
			c.Abort()
			return
		}

		key := fmt.Sprintf("rate_limit:%s:%s", clientID, time.Now().Format("2006-01-02-15"))

		count, err := cache.Increment(key, 1)
		if err != nil {
			// If cache fails, continue without rate limiting
			c.Next()
			return
		}

		// Set expiration if this is the first request
		if count == 1 {
			cache.Set(key, count, time.Hour)
		}

		if count > int64(configs.AppConfig.RateLimitPerHour) {
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":     "Rate limit exceeded",
				"limit":     configs.AppConfig.RateLimitPerHour,
				"remaining": 0,
			})
			c.Abort()
			return
		}

		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", configs.AppConfig.RateLimitPerHour))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", configs.AppConfig.RateLimitPerHour-int(count)))

		c.Next()
	}
}

func ValidationMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Content-Type validation for POST/PUT requests
		if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPut {
			contentType := c.GetHeader("Content-Type")
			if !strings.Contains(contentType, "application/json") {
				c.JSON(http.StatusBadRequest, gin.H{"error": "Content-Type must be application/json"})
				c.Abort()
				return
			}
		}
		c.Next()
	}
}
