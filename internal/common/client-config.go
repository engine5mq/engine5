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
		Host:          EnvOr(EnvHost, "localhost"),
		Port:          EnvIntOr(EnvPort, 3535),
		InstanceID:    EnvOr(EnvInstanceID, "demo-client"),
		InstanceGroup: EnvOr(EnvInstanceGroup, "demo-group"),
		Subject:       EnvOr(EnvSubject, "demo.subject"),
		TLSEnabled:    EnvBoolOr(EnvClientTLS, false),
	}
}

func GetClientConfig() *ClientConfig {
	clientConfigOnce.Do(func() {
		clientConfig = ReadClientConfigFromEnv()
	})
	return clientConfig
}
