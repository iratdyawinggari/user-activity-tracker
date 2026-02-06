package services

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"user-activity-tracker/configs"
	"user-activity-tracker/internal/database"
	"user-activity-tracker/internal/models"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db *gorm.DB
}

func NewAuthService() *AuthService {
	return &AuthService{
		db: database.GetDBManager().WriteDB,
	}
}

type Claims struct {
	ClientID string `json:"client_id"`
	APIKey   string `json:"api_key"`
	jwt.RegisteredClaims
}

func (s *AuthService) GenerateToken(clientID, apiKey string) (string, error) {
	expirationTime := time.Now().Add(configs.AppConfig.JWTTTL)

	claims := &Claims{
		ClientID: clientID,
		APIKey:   apiKey,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "activity-tracker",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(configs.AppConfig.JWTSecret))
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(configs.AppConfig.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	// Check if token is blacklisted
	var blacklist models.JWTBlacklist
	if err := s.db.Where("token = ?", tokenString).First(&blacklist).Error; err == nil {
		return nil, errors.New("token has been revoked")
	}

	return claims, nil
}

func (s *AuthService) RevokeToken(tokenString string) error {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return err
	}

	blacklist := models.JWTBlacklist{
		Token:     tokenString,
		ExpiresAt: claims.ExpiresAt.Time,
	}

	return s.db.Create(&blacklist).Error
}

func (s *AuthService) ValidateAPIKey(apiKey string) (*models.Client, error) {
	var client models.Client
	if err := s.db.Where("api_key = ?", apiKey).First(&client).Error; err != nil {
		return nil, errors.New("invalid API key")
	}
	return &client, nil
}

func (s *AuthService) HashAPIKey(apiKey string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(apiKey), bcrypt.DefaultCost)
	return string(bytes), err
}

func (s *AuthService) CheckAPIKeyHash(apiKey, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(apiKey))
	return err == nil
}

// Data encryption functions
func (s *AuthService) EncryptData(data string) (string, error) {
	key := []byte(configs.AppConfig.JWTSecret)[:32]

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	ciphertext := make([]byte, aes.BlockSize+len(data))
	iv := ciphertext[:aes.BlockSize]
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	stream := cipher.NewCFBEncrypter(block, iv)
	stream.XORKeyStream(ciphertext[aes.BlockSize:], []byte(data))

	return base64.URLEncoding.EncodeToString(ciphertext), nil
}

func (s *AuthService) DecryptData(encrypted string) (string, error) {
	key := []byte(configs.AppConfig.JWTSecret)[:32]

	ciphertext, err := base64.URLEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	if len(ciphertext) < aes.BlockSize {
		return "", errors.New("ciphertext too short")
	}

	iv := ciphertext[:aes.BlockSize]
	ciphertext = ciphertext[aes.BlockSize:]

	stream := cipher.NewCFBDecrypter(block, iv)
	stream.XORKeyStream(ciphertext, ciphertext)

	return string(ciphertext), nil
}

func (s *AuthService) CheckIPWhitelist(client *models.Client, ip string) bool {
	if client.IPWhitelist == "" {
		return true // No whitelist configured, allow all
	}

	allowedIPs := strings.Split(client.IPWhitelist, ",")
	for _, allowedIP := range allowedIPs {
		if strings.TrimSpace(allowedIP) == ip {
			return true
		}
	}

	return false
}

func (s *AuthService) GetClientIPv4(c *gin.Context) string {
	ip := c.ClientIP()

	switch ip {
	case "::1":
		return "127.0.0.1"
	default:
		if strings.HasPrefix(ip, "::ffff:") {
			return ip[7:]
		}
	}

	return ip
}
