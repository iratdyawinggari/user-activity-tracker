package database

import (
	"log"
	"sync"
	"time"

	"user-activity-tracker/configs"
	"user-activity-tracker/internal/models"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type DBManager struct {
	WriteDB      *gorm.DB
	ReadDBs      []*gorm.DB
	CurrentShard int
	shardMutex   sync.RWMutex
}

var (
	instance *DBManager
	once     sync.Once
)

func GetDBManager() *DBManager {
	once.Do(func() {
		instance = &DBManager{
			ReadDBs:      make([]*gorm.DB, 0),
			CurrentShard: 0,
		}
		instance.initialize()
	})
	return instance
}

func (m *DBManager) initialize() {
	// Connect to main write database
	writeDB, err := gorm.Open(mysql.Open(configs.AppConfig.DatabaseURL), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		log.Fatal("Failed to connect to write database:", err)
	}
	m.WriteDB = writeDB

	// Auto-migrate models
	err = m.WriteDB.AutoMigrate(
		&models.Client{},
		&models.APILogs{},
		&models.DailyUsage{},
		&models.JWTBlacklist{},
		&models.ShardMapping{},
	)
	if err != nil {
		log.Fatal("Failed to auto-migrate database:", err)
	}

	// Set up connection pool
	sqlDB, err := m.WriteDB.DB()
	if err == nil {
		sqlDB.SetMaxIdleConns(10)
		sqlDB.SetMaxOpenConns(100)
		sqlDB.SetConnMaxLifetime(time.Hour)
	}

	// Initialize read replicas (simulated - in production these would be different hosts)
	for i := 0; i < 2; i++ { // Two read replicas
		// In production, use different connection strings for replicas
		readDB, err := gorm.Open(mysql.Open(configs.AppConfig.DatabaseURL), &gorm.Config{
			Logger: logger.Default.LogMode(logger.Info),
		})
		if err != nil {
			log.Printf("Warning: Failed to connect to read replica %d: %v", i, err)
			continue
		}
		m.ReadDBs = append(m.ReadDBs, readDB)
	}

	log.Println("Database connection established successfully")
}

// GetReadDB returns a read replica using round-robin
func (m *DBManager) GetReadDB() *gorm.DB {
	m.shardMutex.Lock()
	defer m.shardMutex.Unlock()

	if len(m.ReadDBs) == 0 {
		return m.WriteDB
	}

	db := m.ReadDBs[m.CurrentShard]
	m.CurrentShard = (m.CurrentShard + 1) % len(m.ReadDBs)
	return db
}

// GetShardForClient returns which shard a client should use
func (m *DBManager) GetShardForClient(clientID string) int {
	// Simple consistent hashing for shard assignment
	hash := 0
	for _, char := range clientID {
		hash += int(char)
	}
	return hash % configs.AppConfig.ShardCount
}

// GetShardConnection returns a connection for a specific shard
func (m *DBManager) GetShardConnection(shardID int) (*gorm.DB, error) {
	// In production, this would connect to different database instances
	// For this implementation, we'll use the same DB but different tables
	return m.WriteDB, nil
}

// BatchInsertHits efficiently inserts multiple API hits
func (m *DBManager) BatchInsertHits(hits []models.APILogs) error {
	return m.WriteDB.CreateInBatches(hits, 1000).Error
}
