package common

import "sync"

type ClientConfig struct {
	Host          string
	Port          int
	InstanceID    string
	InstanceGroup string
	Subject       string
	TLSEnabled    bool
}

var (
	clientConfigOnce sync.Once
	clientConfig     *ClientConfig
)

func ReadClientConfigFromEnv() *ClientConfig {
	return &ClientConfig{
		Host:          EnvOr("E5_HOST", "localhost"),
		Port:          EnvIntOr("E5_PORT", 3535),
		InstanceID:    EnvOr("E5_INSTANCE_ID", "demo-client"),
		InstanceGroup: EnvOr("E5_INSTANCE_GROUP", "demo-group"),
		Subject:       EnvOr("E5_SUBJECT", "demo.subject"),
		TLSEnabled:    EnvBoolOr("E5_TLS", false),
	}
}

func GetClientConfig() *ClientConfig {
	clientConfigOnce.Do(func() {
		clientConfig = ReadClientConfigFromEnv()
	})
	return clientConfig
}
