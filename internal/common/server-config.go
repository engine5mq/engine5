package common

import (
	"os"
	"sync"
)

type ServerTLSSettings struct {
	CertFile    string
	KeyFile     string
	CAFile      string
	RequireAuth bool
	ServerName  string
}

type ServerAuthSettings struct {
	Secret                string
	RequireAuth           bool
	ClientPermissionsJSON string
}

type ServerExhaustSettings struct {
	Env            string
	LogLevel       string
	LogFormat      string
	IncludeContent bool
	EnableTap      bool
	Port           string
	Key            string
	TLSEnabled     bool
}

type ServerConfig struct {
	Port                 int
	EnableTLS            bool
	MaxConnections       int
	ConnectionTimeoutSec int
	TLS                  ServerTLSSettings
	Auth                 ServerAuthSettings
	Exhaust              ServerExhaustSettings
}

var (
	serverConfigOnce sync.Once
	serverConfig     *ServerConfig
)

func ReadServerConfigFromEnv() *ServerConfig {
	enableTLS := EnvBoolOr("ENABLE_TLS", true)
	exhaustTLS := enableTLS
	if value, ok := os.LookupEnv("E5_EXHAUST_TLS"); ok {
		exhaustTLS = EnvBoolOr("E5_EXHAUST_TLS", enableTLS)
		if value == "" {
			exhaustTLS = enableTLS
		}
	}

	return &ServerConfig{
		Port:                 EnvIntOr("E5_PORT", 3535),
		EnableTLS:            enableTLS,
		MaxConnections:       EnvIntOr("MAX_CONNECTIONS", 1000),
		ConnectionTimeoutSec: EnvIntOr("CONNECTION_TIMEOUT", 86400),
		TLS: ServerTLSSettings{
			CertFile:    EnvOr("TLS_CERT_FILE", "server.crt"),
			KeyFile:     EnvOr("TLS_KEY_FILE", "server.key"),
			CAFile:      EnvOr("TLS_CA_FILE", "ca.crt"),
			RequireAuth: EnvBoolOr("TLS_REQUIRE_CLIENT_AUTH", false),
			ServerName:  EnvOr("TLS_SERVER_NAME", "localhost"),
		},
		Auth: ServerAuthSettings{
			Secret:                os.Getenv("AUTH_SECRET"),
			RequireAuth:           EnvBoolOr("REQUIRE_AUTH", true),
			ClientPermissionsJSON: os.Getenv("CLIENT_PERMISSIONS"),
		},
		Exhaust: ServerExhaustSettings{
			Env:            EnvOr("E5_ENV", "development"),
			LogLevel:       os.Getenv("E5_LOG_LEVEL"),
			LogFormat:      os.Getenv("E5_LOG_FORMAT"),
			IncludeContent: EnvBoolOr("E5_EXHAUST_INCLUDE_CONTENT", false),
			EnableTap:      EnvBoolOr("E5_EXHAUST_ENABLE", false),
			Port:           EnvOr("E5_EXHAUST_PORT", "3536"),
			Key:            os.Getenv("E5_EXHAUST_KEY"),
			TLSEnabled:     exhaustTLS,
		},
	}
}

func GetServerConfig() *ServerConfig {
	serverConfigOnce.Do(func() {
		serverConfig = ReadServerConfigFromEnv()
	})
	return serverConfig
}
