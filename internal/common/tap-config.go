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
		Host:      EnvOr(EnvExhaustHost, "localhost"),
		Port:      EnvOr(EnvExhaustPort, "3536"),
		Key:       os.Getenv(EnvExhaustKey),
		UseTLS:    EnvBoolOr(EnvExhaustTLS, true),
		CAFile:    os.Getenv(EnvExhaustCAFile),
		Reconnect: EnvBoolOr(EnvExhaustReconnect, true),
	}
}

func GetTapConfig() *TapConfig {
	tapConfigOnce.Do(func() {
		tapConfig = ReadTapConfigFromEnv()
	})
	return tapConfig
}
