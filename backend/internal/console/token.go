package console

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"
)

type TokenInfo struct {
	ContainerName string
	CreatedAt     time.Time
	Used          bool
}

var (
	tokens = make(map[string]*TokenInfo)
	mu     sync.RWMutex
)

func GenerateToken(containerName string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	
	mu.Lock()
	tokens[token] = &TokenInfo{
		ContainerName: containerName,
		CreatedAt:     time.Now(),
		Used:          false,
	}
	mu.Unlock()
	
	go func() {
		time.Sleep(10 * time.Minute)
		mu.Lock()
		delete(tokens, token)
		mu.Unlock()
	}()
	
	return token, nil
}

func ValidateToken(token string) (string, bool, string) {
	mu.RLock()
	defer mu.RUnlock()
	
	info, exists := tokens[token]
	if !exists {
		return "", false, "令牌无效或已过期"
	}
	
	if info.Used {
		return "", false, "令牌已被使用，无法重复访问"
	}
	
	if time.Since(info.CreatedAt) > 10*time.Minute {
		return "", false, "令牌已过期（有效期10分钟）"
	}
	
	return info.ContainerName, true, ""
}

func ValidateAndConsume(token string) (string, bool) {
	mu.Lock()
	defer mu.Unlock()
	
	info, exists := tokens[token]
	if !exists {
		return "", false
	}
	
	if info.Used {
		return "", false
	}
	
	if time.Since(info.CreatedAt) > 10*time.Minute {
		delete(tokens, token)
		return "", false
	}
	
	delete(tokens, token)
	
	return info.ContainerName, true
}

