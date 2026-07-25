package common

import (
	"os"
	"sync"
)

type TapConfig struct {
	Host      string
	Port      string
	Key       string
	UseTLS    bool
	CAFile    string
	Reconnect bool
}

var (
	tapConfigOnce sync.Once
	tapConfig     *TapConfig
)

func ReadTapConfigFromEnv() *TapConfig {
	return &TapConfig{
		Host:      EnvOr("E5_EXHAUST_HOST", "localhost"),
		Port:      EnvOr("E5_EXHAUST_PORT", "3536"),
		Key:       os.Getenv("E5_EXHAUST_KEY"),
		UseTLS:    EnvBoolOr("E5_EXHAUST_TLS", true),
		CAFile:    os.Getenv("E5_EXHAUST_CA_FILE"),
		Reconnect: EnvBoolOr("E5_EXHAUST_RECONNECT", true),
	}
}

func GetTapConfig() *TapConfig {
	tapConfigOnce.Do(func() {
		tapConfig = ReadTapConfigFromEnv()
	})
	return tapConfig
}
