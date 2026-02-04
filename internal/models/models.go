package models

import (
	"time"
)

type Client struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClientID    string    `gorm:"uniqueIndex;not null;size:100" json:"client_id"`
	Name        string    `gorm:"not null;size:255" json:"name"`
	Email       string    `gorm:"not null;uniqueIndex;size:255" json:"email"`
	APIKey      string    `gorm:"uniqueIndex;not null;size:255" json:"api_key"`
	IPWhitelist string    `gorm:"type:text" json:"ip_whitelist"` // comma-separated IPs
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type APILogs struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ClientID  string    `gorm:"index:idx_client_timestamp;not null" json:"client_id"`
	Endpoint  string    `gorm:"not null;size:500" json:"endpoint"`
	IPAddress string    `gorm:"not null;size:45" json:"ip_address"` // IPv6 compatible
	Timestamp time.Time `gorm:"index:idx_client_timestamp;not null" json:"timestamp"`
	CreatedAt time.Time `json:"created_at"`
	Client    Client    `gorm:"foreignKey:ClientID;references:ClientID;constraint:OnDelete:CASCADE"`
}

type DailyUsage struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ClientID     string    `gorm:"uniqueIndex:idx_client_date;not null" json:"client_id"`
	Date         time.Time `gorm:"uniqueIndex:idx_client_date;type:date;not null" json:"date"`
	RequestCount int64     `gorm:"not null;default:0" json:"request_count"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type JWTBlacklist struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Token     string    `gorm:"uniqueIndex;not null" json:"token"`
	ExpiresAt time.Time `gorm:"index;not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

type ShardMapping struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	ClientID  string    `gorm:"uniqueIndex;not null" json:"client_id"`
	ShardID   uint      `gorm:"not null" json:"shard_id"`
	CreatedAt time.Time `json:"created_at"`
	Client    Client    `gorm:"foreignKey:ClientID;references:ClientID;constraint:OnDelete:CASCADE"`
}
