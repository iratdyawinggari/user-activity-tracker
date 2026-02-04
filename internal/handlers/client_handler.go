package handlers

import (
	"fmt"
	"net/http"
	"time"

	"user-activity-tracker/configs"
	"user-activity-tracker/internal/cache"
	"user-activity-tracker/internal/database"
	"user-activity-tracker/internal/models"
	"user-activity-tracker/internal/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ClientHandler struct {
	db          *gorm.DB
	authService *services.AuthService
	cache       *cache.CacheManager
}

func NewClientHandler(authService *services.AuthService) *ClientHandler {
	return &ClientHandler{
		db:          database.GetDBManager().WriteDB,
		authService: authService,
		cache:       cache.GetCacheManager(),
	}
}

// RegisterClient handles client registration
// @Summary Register a new client
// @Description Register a new client with name, email, and generate API key
// @Tags clients
// @Accept json
// @Produce json
// @Param request body RegisterRequest true "Client registration data"
// @Success 201 {object} RegisterResponse
// @Failure 400 {object} ErrorResponse
// @Failure 409 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/register [post]
func (h *ClientHandler) RegisterClient(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	// Validate input
	if req.Name == "" || req.Email == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Name and email are required"})
		return
	}

	// Check if email already exists
	var existingClient models.Client
	if err := h.db.Where("email = ?", req.Email).First(&existingClient).Error; err == nil {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Email already registered"})
		return
	}

	// Generate unique client ID and API key
	clientID := uuid.New().String()
	apiKey := uuid.New().String()

	// Hash API key for storage
	hashedAPIKey, err := h.authService.HashAPIKey(apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate API key"})
		return
	}

	// Create client
	client := models.Client{
		ClientID:    clientID,
		Name:        req.Name,
		Email:       req.Email,
		APIKey:      hashedAPIKey,
		IPWhitelist: req.IPWhitelist,
	}

	if err := h.db.Create(&client).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to register client"})
		return
	}

	// Create shard mapping
	shardID := database.GetDBManager().GetShardForClient(clientID)
	shardMapping := models.ShardMapping{
		ClientID: clientID,
		ShardID:  uint(shardID),
	}
	h.db.Create(&shardMapping)

	// Generate JWT token
	token, err := h.authService.GenerateToken(clientID, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to generate token"})
		return
	}

	// Encrypt API key for response
	encryptedAPIKey, _ := h.authService.EncryptData(apiKey)

	response := RegisterResponse{
		ClientID:  clientID,
		Name:      client.Name,
		Email:     client.Email,
		APIKey:    encryptedAPIKey,
		Token:     token,
		CreatedAt: time.Now(),
	}

	c.JSON(http.StatusCreated, response)
}

// RecordLog handles API hit logging
// @Summary Record an API hit
// @Description Record an API hit with client information
// @Tags logs
// @Accept json
// @Produce json
// @Param request body LogRequest true "API hit data"
// @Security ApiKeyAuth
// @Success 202 {object} SuccessResponse
// @Failure 400 {object} ErrorResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/logs [post]
func (h *ClientHandler) RecordLog(c *gin.Context) {
	var req LogRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid request body"})
		return
	}

	clientID, _ := c.Get("client_id")
	ipAddress := c.ClientIP()

	// Create API hit record
	apiHit := models.APILogs{
		ClientID:  clientID.(string),
		Endpoint:  req.Endpoint,
		IPAddress: ipAddress,
		Timestamp: time.Now(),
	}

	// Use batch insert for better performance
	// In production, this would be queued and processed asynchronously
	if err := h.db.Create(&apiHit).Error; err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to record log"})
		return
	}

	// Update cache counters atomically
	dailyKey := fmt.Sprintf("counter:daily:%s:%s", clientID, time.Now().Format("2006-01-02"))
	totalKey := fmt.Sprintf("counter:total:%s", clientID)

	h.cache.Increment(dailyKey, 1)
	h.cache.Increment(totalKey, 1)

	// Publish update for real-time notifications
	h.cache.PublishUpdate(clientID.(string))

	c.JSON(http.StatusAccepted, SuccessResponse{
		Message: "Log recorded successfully",
		Data:    map[string]interface{}{"hit_id": apiHit.ID},
	})
}

// GetDailyUsage returns daily usage for last 7 days
// @Summary Get daily usage
// @Description Get total daily requests per client for the last 7 days
// @Tags usage
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} DailyUsageResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/usage/daily [get]
func (h *ClientHandler) GetDailyUsage(c *gin.Context) {
	clientID, _ := c.Get("client_id")

	// Try cache first
	cacheKey := fmt.Sprintf("usage:daily:%s", clientID)
	var cachedResponse DailyUsageResponse
	if found, err := h.cache.Get(cacheKey, &cachedResponse); found && err == nil {
		c.JSON(http.StatusOK, cachedResponse)
		return
	}

	// Calculate date range
	endDate := time.Now()
	startDate := endDate.AddDate(0, 0, -6) // Last 7 days

	var dailyUsage []struct {
		Date         string `json:"date"`
		RequestCount int64  `json:"request_count"`
	}

	// Query from database (using read replica)
	readDB := database.GetDBManager().GetReadDB()
	err := readDB.Model(&models.DailyUsage{}).
		Select("DATE_FORMAT(date, '%Y-%m-%d') as date, SUM(request_count) as request_count").
		Where("client_id = ? AND date >= ? AND date <= ?", clientID, startDate.Format("2006-01-02"), endDate.Format("2006-01-02")).
		Group("date").
		Order("date DESC").
		Scan(&dailyUsage).Error

	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch usage data"})
		return
	}

	// Fill in missing days with zero
	response := h.fillMissingDays(dailyUsage, startDate, endDate)

	// Cache the result
	h.cache.Set(cacheKey, response, configs.AppConfig.CacheTTL)

	c.JSON(http.StatusOK, response)
}

func (h *ClientHandler) fillMissingDays(data []struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
}, startDate, endDate time.Time) DailyUsageResponse {
	result := DailyUsageResponse{
		ClientID:  "",
		StartDate: startDate.Format("2006-01-02"),
		EndDate:   endDate.Format("2006-01-02"),
		Usage:     make([]DayUsage, 0),
	}

	// Create a map for easy lookup
	dataMap := make(map[string]int64)
	for _, item := range data {
		dataMap[item.Date] = item.RequestCount
	}

	// Generate all dates in range
	for d := startDate; !d.After(endDate); d = d.AddDate(0, 0, 1) {
		dateStr := d.Format("2006-01-02")
		count := dataMap[dateStr]
		result.Usage = append(result.Usage, DayUsage{
			Date:         dateStr,
			RequestCount: count,
		})
	}

	return result
}

// GetTopClients returns top 3 clients with highest requests in last 24 hours
// @Summary Get top clients
// @Description Get top 3 clients with highest total requests in last 24 hours
// @Tags usage
// @Produce json
// @Security ApiKeyAuth
// @Success 200 {object} TopClientsResponse
// @Failure 401 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /api/usage/top [get]
func (h *ClientHandler) GetTopClients(c *gin.Context) {
	// Try cache first with prefetch mechanism
	cacheKey := "usage:top:last24h"
	var cachedResponse TopClientsResponse

	if found, err := h.cache.Get(cacheKey, &cachedResponse); found && err == nil {
		// Check if cache is stale (more than 5 minutes old)
		if time.Since(cachedResponse.GeneratedAt) < 5*time.Minute {
			c.JSON(http.StatusOK, cachedResponse)
			return
		}
		// Cache is stale, refresh in background
		go h.refreshTopClientsCache()
	} else {
		// Cache miss, fetch and cache
		h.refreshTopClientsCache()
	}

	// Get fresh data
	response, err := h.fetchTopClients()
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to fetch top clients"})
		return
	}

	c.JSON(http.StatusOK, response)
}

func (h *ClientHandler) refreshTopClientsCache() {
	data, err := h.fetchTopClients()
	if err == nil {
		h.cache.Set("usage:top:last24h", data, configs.AppConfig.CacheTTL)
	}
}

func (h *ClientHandler) fetchTopClients() (TopClientsResponse, error) {
	twentyFourHoursAgo := time.Now().Add(-24 * time.Hour)

	var topClients []struct {
		ClientID     string `json:"client_id"`
		Name         string `json:"name"`
		RequestCount int64  `json:"request_count"`
	}

	readDB := database.GetDBManager().GetReadDB()
	err := readDB.Model(&models.APILogs{}).
		Select("clients.client_id, clients.name, COUNT(api_hits.id) as request_count").
		Joins("JOIN clients ON clients.client_id = api_hits.client_id").
		Where("api_hits.timestamp >= ?", twentyFourHoursAgo).
		Group("clients.client_id, clients.name").
		Order("request_count DESC").
		Limit(3).
		Scan(&topClients).Error

	if err != nil {
		return TopClientsResponse{}, err
	}

	return TopClientsResponse{
		Period:       "last_24_hours",
		GeneratedAt:  time.Now(),
		TopClients:   topClients,
		TotalClients: len(topClients),
	}, nil
}

// Request/Response structures
type RegisterRequest struct {
	Name        string `json:"name" binding:"required"`
	Email       string `json:"email" binding:"required,email"`
	IPWhitelist string `json:"ip_whitelist"`
}

type RegisterResponse struct {
	ClientID  string    `json:"client_id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	APIKey    string    `json:"api_key"`
	Token     string    `json:"token"`
	CreatedAt time.Time `json:"created_at"`
}

type LogRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
}

type SuccessResponse struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type DailyUsageResponse struct {
	ClientID  string     `json:"client_id"`
	StartDate string     `json:"start_date"`
	EndDate   string     `json:"end_date"`
	Usage     []DayUsage `json:"usage"`
}

type DayUsage struct {
	Date         string `json:"date"`
	RequestCount int64  `json:"request_count"`
}

type TopClientsResponse struct {
	Period      string    `json:"period"`
	GeneratedAt time.Time `json:"generated_at"`
	TopClients  []struct {
		ClientID     string `json:"client_id"`
		Name         string `json:"name"`
		RequestCount int64  `json:"request_count"`
	} `json:"top_clients"`
	TotalClients int `json:"total_clients"`
}
