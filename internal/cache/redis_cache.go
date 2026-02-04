package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"user-activity-tracker/configs"

	"github.com/go-redis/redis/v8"
	"github.com/patrickmn/go-cache"
)

type CacheManager struct {
	redisClient *redis.Client
	localCache  *cache.Cache
	pubSub      *redis.PubSub
	ctx         context.Context
	mu          sync.RWMutex
}

var (
	instance *CacheManager
	once     sync.Once
)

type CacheItem struct {
	Key        string      `json:"key"`
	Value      interface{} `json:"value"`
	Expiration time.Time   `json:"expiration"`
	Version    int         `json:"version"`
}

func GetCacheManager() *CacheManager {
	once.Do(func() {
		instance = &CacheManager{
			ctx:        context.Background(),
			localCache: cache.New(5*time.Minute, 10*time.Minute),
		}
		instance.initialize()
	})
	return instance
}

func (cm *CacheManager) initialize() {
	// Initialize Redis client
	opts, err := redis.ParseURL(configs.AppConfig.RedisURL)
	if err != nil {
		opts = &redis.Options{
			Addr:     configs.AppConfig.RedisURL,
			Password: "", // no password set
			DB:       0,  // use default DB
		}
	}

	cm.redisClient = redis.NewClient(opts)

	// Test Redis connection
	ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
	defer cancel()

	if err := cm.redisClient.Ping(ctx).Err(); err != nil {
		log.Printf("Redis connection failed, using local cache only: %v", err)
		cm.redisClient = nil
	} else {
		log.Println("Redis connection established successfully")

		// Initialize Pub/Sub
		cm.pubSub = cm.redisClient.Subscribe(cm.ctx, "usage_updates")
		go cm.listenForUpdates()
	}
}

func (cm *CacheManager) listenForUpdates() {
	if cm.pubSub == nil {
		return
	}

	ch := cm.pubSub.Channel()
	for msg := range ch {
		cm.handleUpdateMessage(msg.Payload)
	}
}

func (cm *CacheManager) handleUpdateMessage(payload string) {
	var update struct {
		Action    string `json:"action"`
		ClientID  string `json:"client_id"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal([]byte(payload), &update); err != nil {
		log.Printf("Failed to parse update message: %v", err)
		return
	}

	// Invalidate related caches
	cacheKeys := []string{
		fmt.Sprintf("usage:daily:%s", update.ClientID),
		"usage:top:last24h",
		fmt.Sprintf("client:%s", update.ClientID),
	}

	for _, key := range cacheKeys {
		cm.Delete(key)
	}
}

func (cm *CacheManager) Set(key string, value interface{}, ttl time.Duration) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Store in local cache
	cm.localCache.Set(key, value, ttl)

	// Store in Redis if available
	if cm.redisClient != nil {
		data, err := json.Marshal(value)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
		defer cancel()

		return cm.redisClient.Set(ctx, key, data, ttl).Err()
	}

	return nil
}

func (cm *CacheManager) Get(key string, target interface{}) (bool, error) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// Try local cache first
	if val, found := cm.localCache.Get(key); found {
		// Type assertion
		data, err := json.Marshal(val)
		if err != nil {
			return false, err
		}
		return true, json.Unmarshal(data, target)
	}

	// Try Redis if available
	if cm.redisClient != nil {
		ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
		defer cancel()

		data, err := cm.redisClient.Get(ctx, key).Bytes()
		if err == redis.Nil {
			return false, nil
		} else if err != nil {
			return false, err
		}

		// Store in local cache for faster subsequent access
		cm.localCache.Set(key, data, 5*time.Minute)

		return true, json.Unmarshal(data, target)
	}

	return false, nil
}

func (cm *CacheManager) Delete(key string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// Delete from local cache
	cm.localCache.Delete(key)

	// Delete from Redis if available
	if cm.redisClient != nil {
		ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
		defer cancel()
		return cm.redisClient.Del(ctx, key).Err()
	}

	return nil
}

func (cm *CacheManager) Increment(key string, value int64) (int64, error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if cm.redisClient != nil {
		ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
		defer cancel()
		return cm.redisClient.IncrBy(ctx, key, value).Result()
	}

	// Fallback to local cache
	var current int64
	if val, found := cm.localCache.Get(key); found {
		current = val.(int64)
	}
	current += value
	cm.localCache.Set(key, current, cache.DefaultExpiration)
	return current, nil
}

func (cm *CacheManager) PublishUpdate(clientID string) {
	if cm.redisClient == nil {
		return
	}

	update := map[string]interface{}{
		"action":    "usage_updated",
		"client_id": clientID,
		"timestamp": time.Now().Unix(),
	}

	data, _ := json.Marshal(update)
	ctx, cancel := context.WithTimeout(cm.ctx, 5*time.Second)
	defer cancel()

	cm.redisClient.Publish(ctx, "usage_updates", data)
}

// Cache warming functions
func (cm *CacheManager) WarmUsageCache() {
	// Pre-warm commonly accessed cache
	keysToWarm := []string{
		"usage:top:last24h",
		"system:stats",
	}

	for _, key := range keysToWarm {
		// In a real implementation, you would fetch and cache the data
		cm.Set(key, map[string]interface{}{"warming": true}, 30*time.Second)
	}
}

func (cm *CacheManager) IsAvailable() bool {
	return cm.redisClient != nil
}
