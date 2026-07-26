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
	enableTLS := EnvBoolOr(EnvEnableTLS, true)
	exhaustTLS := enableTLS
	if value, ok := os.LookupEnv(EnvExhaustTLS); ok {
		exhaustTLS = EnvBoolOr(EnvExhaustTLS, enableTLS)
		if value == "" {
			exhaustTLS = enableTLS
		}
	}

	return &ServerConfig{
		Port:                 EnvIntOr(EnvPort, 3535),
		EnableTLS:            enableTLS,
		MaxConnections:       EnvIntOr(EnvMaxConnections, 1000),
		ConnectionTimeoutSec: EnvIntOr(EnvConnectionTimeout, 86400),
		TLS: ServerTLSSettings{
			CertFile:    EnvOr(EnvTLSCertFile, "server.crt"),
			KeyFile:     EnvOr(EnvTLSKeyFile, "server.key"),
			CAFile:      EnvOr(EnvTLSCAFile, "ca.crt"),
			RequireAuth: EnvBoolOr(EnvTLSRequireClientAuth, false),
			ServerName:  EnvOr(EnvTLSServerName, "localhost"),
		},
		Auth: ServerAuthSettings{
			Secret:                os.Getenv(EnvAuthSecret),
			RequireAuth:           EnvBoolOr(EnvRequireAuth, true),
			ClientPermissionsJSON: os.Getenv(EnvClientPermissions),
		},
		Exhaust: ServerExhaustSettings{
			Env:            EnvOr(EnvExhaustEnv, "development"),
			LogLevel:       os.Getenv(EnvExhaustLogLevel),
			LogFormat:      os.Getenv(EnvExhaustLogFormat),
			IncludeContent: EnvBoolOr(EnvExhaustIncludeContent, false),
			EnableTap:      EnvBoolOr(EnvExhaustEnable, false),
			Port:           EnvOr(EnvExhaustPort, "3536"),
			Key:            os.Getenv(EnvExhaustKey),
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
