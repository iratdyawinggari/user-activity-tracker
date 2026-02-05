package models

import "time"

// Clients
type Client struct {
	ID          uint   `gorm:"primaryKey;autoIncrement"`
	ClientID    string `gorm:"type:varchar(100);uniqueIndex;not null"`
	Name        string `gorm:"type:varchar(255);not null"`
	Email       string `gorm:"type:varchar(255);uniqueIndex;not null"`
	APIKey      string `gorm:"type:varchar(255);uniqueIndex;not null"`
	IPWhitelist string `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func (Client) TableName() string {
	return "clients"
}

// API Logs (partitioned table)
type APILogs struct {
	ID        uint64    `gorm:"primaryKey;autoIncrement"`
	ClientID  string    `gorm:"type:varchar(100);index:idx_client_time;not null"`
	Endpoint  string    `gorm:"type:varchar(500);not null"`
	IPAddress string    `gorm:"type:varchar(45);not null"`
	Timestamp time.Time `gorm:"index:idx_timestamp;not null"`
	CreatedAt time.Time
}

func (APILogs) TableName() string {
	return "api_logs"
}

// Daily Usage Aggregation
type DailyUsage struct {
	ID           uint      `gorm:"primaryKey;autoIncrement"`
	ClientID     string    `gorm:"type:varchar(100);uniqueIndex:idx_client_date;not null"`
	Date         time.Time `gorm:"type:date;uniqueIndex:idx_client_date"`
	RequestCount uint64    `gorm:"not null;default:0"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (DailyUsage) TableName() string {
	return "daily_usage"
}

// JWT Blacklist
type JWTBlacklist struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	Token     string    `gorm:"type:text;not null"`
	ExpiresAt time.Time `gorm:"index;not null"`
	CreatedAt time.Time
}

func (JWTBlacklist) TableName() string {
	return "jwt_blacklist"
}

// Shard Mapping
type ShardMapping struct {
	ID        uint   `gorm:"primaryKey;autoIncrement"`
	ClientID  string `gorm:"type:varchar(100);uniqueIndex;not null"`
	ShardID   uint   `gorm:"index;not null"`
	CreatedAt time.Time
}

func (ShardMapping) TableName() string {
	return "shard_mapping"
}
