package main

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	JWTSecret      []byte
	RequireAuth    bool
	TokenExpiry    time.Duration
	AllowedClients map[string]ClientPermissions
}

// ClientPermissions defines what a client can do
type ClientPermissions struct {
	CanPublish      bool     `json:"can_publish"`
	CanSubscribe    bool     `json:"can_subscribe"`
	CanRequest      bool     `json:"can_request"`
	AllowedSubjects []string `json:"allowed_subjects"`
	RateLimit       int      `json:"rate_limit"` // requests per minute
}

// AuthToken represents a JWT-like token
type AuthToken struct {
	ClientID    string            `json:"client_id"`
	Permissions ClientPermissions `json:"permissions"`
	IssuedAt    time.Time         `json:"issued_at"`
	ExpiresAt   time.Time         `json:"expires_at"`
}

// AuthenticatedClient extends ConnectedClient with auth info
type AuthenticatedClient struct {
	*ConnectedClient
	Token       *AuthToken
	RateLimiter *RateLimiter
	IsAuth      bool
}

// RateLimiter implements simple rate limiting
type RateLimiter struct {
	requests   []time.Time
	limit      int
	timeWindow time.Duration
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(limit int) *RateLimiter {
	return &RateLimiter{
		requests:   make([]time.Time, 0),
		limit:      limit,
		timeWindow: time.Minute,
	}
}

// Allow checks if request is allowed
func (rl *RateLimiter) Allow() bool {
	now := time.Now()

	// Remove old requests outside the time window
	validRequests := make([]time.Time, 0)
	for _, reqTime := range rl.requests {
		if now.Sub(reqTime) < rl.timeWindow {
			validRequests = append(validRequests, reqTime)
		}
	}
	rl.requests = validRequests

	// Check if we're under the limit
	if len(rl.requests) >= rl.limit {
		return false
	}

	// Add current request
	rl.requests = append(rl.requests, now)
	return true
}

// LoadAuthConfig loads authentication configuration
func LoadAuthConfig() *AuthConfig {
	secret := os.Getenv("AUTH_SECRET")
	if secret == "" {
		secret = generateRandomSecret()
		fmt.Printf("Generated new AUTH_SECRET: %s\n", secret)
	}

	config := &AuthConfig{
		JWTSecret:      []byte(secret),
		RequireAuth:    getEnvWithDefault("REQUIRE_AUTH", "true") == "true",
		TokenExpiry:    time.Hour * 24, // 24 hours default
		AllowedClients: make(map[string]ClientPermissions),
	}

	// Load client permissions from environment or config file
	clientsConfig := os.Getenv("CLIENT_PERMISSIONS")
	if clientsConfig != "" {
		json.Unmarshal([]byte(clientsConfig), &config.AllowedClients)
	} else {
		// Default permissions
		config.AllowedClients["default"] = ClientPermissions{
			CanPublish:      true,
			CanSubscribe:    true,
			CanRequest:      true,
			AllowedSubjects: []string{"*"}, // all subjects
			RateLimit:       60,            // 60 requests per minute
		}
	}

	return config
}

// GenerateToken creates a new authentication token
func (ac *AuthConfig) GenerateToken(clientID string) (string, error) {
	permissions, exists := ac.AllowedClients[clientID]
	if !exists {
		permissions = ac.AllowedClients["default"]
	}

	token := AuthToken{
		ClientID:    clientID,
		Permissions: permissions,
		IssuedAt:    time.Now(),
		ExpiresAt:   time.Now().Add(ac.TokenExpiry),
	}

	tokenJSON, err := json.Marshal(token)
	if err != nil {
		return "", err
	}

	// Create HMAC signature
	h := hmac.New(sha256.New, ac.JWTSecret)
	h.Write(tokenJSON)
	signature := h.Sum(nil)

	// Combine token and signature
	tokenWithSig := base64.URLEncoding.EncodeToString(tokenJSON) + "." + base64.URLEncoding.EncodeToString(signature)

	return tokenWithSig, nil
}

// ValidateToken validates and decodes a token
func (ac *AuthConfig) ValidateToken(tokenString string) (*AuthToken, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid token format")
	}

	tokenJSON, err := base64.URLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid token encoding")
	}

	signature, err := base64.URLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding")
	}

	// Verify signature
	h := hmac.New(sha256.New, ac.JWTSecret)
	h.Write(tokenJSON)
	expectedSignature := h.Sum(nil)

	if !hmac.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid signature")
	}

	// Decode token
	var token AuthToken
	if err := json.Unmarshal(tokenJSON, &token); err != nil {
		return nil, fmt.Errorf("invalid token data")
	}

	// Check expiration
	if time.Now().After(token.ExpiresAt) {
		return nil, fmt.Errorf("token expired")
	}

	return &token, nil
}

// CheckPermission checks if client has permission for an action
func (ac *AuthenticatedClient) CheckPermission(action string, subject string) bool {
	if !ac.IsAuth {
		return false
	}

	switch action {
	case "publish":
		if !ac.Token.Permissions.CanPublish {
			return false
		}
	case "subscribe":
		if !ac.Token.Permissions.CanSubscribe {
			return false
		}
	case "request":
		if !ac.Token.Permissions.CanRequest {
			return false
		}
	}

	// Check subject permissions
	for _, allowedSubject := range ac.Token.Permissions.AllowedSubjects {
		if allowedSubject == "*" || allowedSubject == subject {
			return true
		}
		// Simple wildcard matching
		if strings.HasSuffix(allowedSubject, "*") {
			prefix := strings.TrimSuffix(allowedSubject, "*")
			if strings.HasPrefix(subject, prefix) {
				return true
			}
		}
	}

	return false
}

func generateRandomSecret() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return base64.URLEncoding.EncodeToString(bytes)
}
