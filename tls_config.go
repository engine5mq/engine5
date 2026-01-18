package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
)

// TLSConfig holds TLS configuration
type TLSConfig struct {
	CertFile    string
	KeyFile     string
	CAFile      string
	RequireAuth bool
	ServerName  string
}

// LoadTLSConfig creates TLS configuration from environment variables
func LoadTLSConfig() *TLSConfig {
	return &TLSConfig{
		CertFile:    getEnvWithDefault("TLS_CERT_FILE", "server.crt"),
		KeyFile:     getEnvWithDefault("TLS_KEY_FILE", "server.key"),
		CAFile:      getEnvWithDefault("TLS_CA_FILE", "ca.crt"),
		RequireAuth: getEnvWithDefault("TLS_REQUIRE_CLIENT_AUTH", "false") == "true",
		ServerName:  getEnvWithDefault("TLS_SERVER_NAME", "localhost"),
	}
}

// CreateTLSConfig creates a TLS configuration for the server
func (tc *TLSConfig) CreateTLSConfig() (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(tc.CertFile, tc.KeyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load key pair: %v", err)
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
		CurvePreferences: []tls.CurveID{
			tls.CurveP256,
			tls.X25519,
		},
	}

	// Client certificate authentication
	if tc.RequireAuth && tc.CAFile != "" {
		caCert, err := os.ReadFile(tc.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read CA file: %v", err)
		}

		caCertPool := x509.NewCertPool()
		if !caCertPool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("failed to parse CA certificate")
		}

		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		tlsConfig.ClientCAs = caCertPool
	}

	return tlsConfig, nil
}

func getEnvWithDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
