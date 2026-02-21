package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// AuthConfig holds authentication configuration
type AuthConfig struct {
	AuthSecret     []byte
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
		AuthSecret:     []byte(secret),
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

// ValidateAuthKey validates the provided auth key against configured keys
// TLS already handles encryption, so we only need simple key validation
func (ac *AuthConfig) ValidateAuthKey(authKey string, clientID string) (ClientPermissions, error) {
	if authKey == "" {
		return ClientPermissions{}, fmt.Errorf("auth key is required")
	}

	// Check if auth key matches the secret (simple validation)
	if authKey != string(ac.AuthSecret) {
		return ClientPermissions{}, fmt.Errorf("invalid auth key")
	}

	// Get permissions for the client
	permissions, exists := ac.AllowedClients[clientID]
	if !exists {
		permissions = ac.AllowedClients["default"]
	}

	return permissions, nil
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
