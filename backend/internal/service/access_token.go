package service

import (
	"crypto/rand"
	"encoding/hex"
	"lxdapi/internal/db"
	"lxdapi/models"
	"time"
)

func generateToken() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func CreateAccessToken(tokenType, target string, duration time.Duration) (*models.AccessToken, error) {
	db.DB.Unscoped().Where("type = ? AND target = ?", tokenType, target).Delete(&models.AccessToken{})

	token := &models.AccessToken{
		Token:     generateToken(),
		Type:      tokenType,
		Target:    target,
		ExpiresAt: time.Now().Add(duration),
	}

	if err := db.DB.Create(token).Error; err != nil {
		return nil, err
	}

	return token, nil
}

func ValidateAccessToken(token string) (*models.AccessToken, error) {
	var accessToken models.AccessToken
	if err := db.DB.Where("token = ? AND expires_at > ?", token, time.Now()).First(&accessToken).Error; err != nil {
		return nil, err
	}

	return &accessToken, nil
}

func CleanExpiredTokens() {
	db.DB.Unscoped().Where("expires_at < ?", time.Now()).Delete(&models.AccessToken{})
}
