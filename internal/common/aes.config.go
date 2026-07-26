package common

import "sync"

// AesConnectionConfig holds the configuration for the AES connection (Other configuration such as Tap and Client connections will be gathered from related configs).
type AesConnectionConfig struct {
	// REST API CONFIG
	RestApiEnable bool
	RestApiHost   string
	RestApiPort   int

	// DB Bağlantısı
	DBHost               string
	DBPort               int
	DBUser               string
	DBPassword           string
	DBName               string
	DBDriver             string
	DBGenerateIfNotExist bool
	DBSSLEnable          bool
	DBShowQueries        bool
}

var (
	aesConnectionConfigOnce sync.Once
	aesConnectionConfig     *AesConnectionConfig
)

func ReadAesConnectionConfigFromEnv() *AesConnectionConfig {
	return &AesConnectionConfig{
		RestApiEnable:        EnvBoolOr(EnvAesRestApiEnable, false),
		RestApiHost:          EnvOr(EnvAesRestApiHost, "localhost"),
		RestApiPort:          EnvIntOr(EnvAesRestApiPort, 8080),
		DBHost:               EnvOr(EnvAesDbHost, "localhost"),
		DBPort:               EnvIntOr(EnvAesDbPort, 5432),
		DBUser:               EnvOr(EnvAesDbUser, "user"),
		DBPassword:           EnvOrAllowEmpty(EnvAesDbPassword, "password"),
		DBName:               EnvOr(EnvAesDbName, "aes_db"),
		DBDriver:             EnvOr(EnvAesDbDriver, "postgres"),
		DBSSLEnable:          EnvBoolOr(EnvAesDbSSLMode, false),
		DBGenerateIfNotExist: EnvBoolOr(EnvAesDbGenerateIfNotExist, true),
		DBShowQueries:        EnvBoolOr(EnvAesDbShowQueries, false),
	}
}

func GetAesConnectionConfig() *AesConnectionConfig {
	aesConnectionConfigOnce.Do(func() {
		aesConnectionConfig = ReadAesConnectionConfigFromEnv()
	})
	return aesConnectionConfig
}
