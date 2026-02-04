package configs

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerPort        string
	DatabaseURL       string
	RedisURL          string
	JWTSecret         string
	JWTTTL            time.Duration
	RateLimitPerHour  int
	CacheTTL          time.Duration
	ShardCount        int
	EnableWebSocket   bool
	EnableIPWhitelist bool
}

var AppConfig *Config

func LoadConfig() error {

	godotenv.Load()

	AppConfig = &Config{
		ServerPort:        getEnv("SERVER_PORT", "8080"),
		DatabaseURL:       getEnv("DATABASE_URL", "root:password@tcp(localhost:3306)/activity_tracker?charset=utf8mb4&parseTime=True&loc=Local"),
		RedisURL:          getEnv("REDIS_URL", "localhost:6379"),
		JWTSecret:         getEnv("JWT_SECRET", "your-secret-key-change-in-production"),
		JWTTTL:            parseDuration(getEnv("JWT_TTL", "24h")),
		RateLimitPerHour:  parseInt(getEnv("RATE_LIMIT_PER_HOUR", "1000")),
		CacheTTL:          parseDuration(getEnv("CACHE_TTL", "1h")),
		ShardCount:        parseInt(getEnv("SHARD_COUNT", "4")),
		EnableWebSocket:   parseBool(getEnv("ENABLE_WEBSOCKET", "true")),
		EnableIPWhitelist: parseBool(getEnv("ENABLE_IP_WHITELIST", "false")),
	}

	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func parseInt(s string) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return i
}

func parseBool(s string) bool {
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Hour
	}
	return d
}

func init() {
	if err := LoadConfig(); err != nil {
		log.Fatal("Failed to load config:", err)
	}
}
