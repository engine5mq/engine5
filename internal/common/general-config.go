package common

import "sync"

type GeneralConfig struct {
	Host           string
	EventPort      int
	ExhaustivePort int
	InstanceID     string
	InstanceGroup  string
	// TLSConfig      *TLSConfig
	ActiveRulePath string
}

var (
	generalConfigOnce sync.Once
	generalConfig     *GeneralConfig
)

func ReadFromEnv() *GeneralConfig {
	return &GeneralConfig{
		Host:           EnvOr(EnvHost, "localhost"),
		EventPort:      EnvIntOr(EnvEventPort, 8080),
		ExhaustivePort: EnvIntOr(EnvExhaustivePort, 8081),
		InstanceID:     EnvOr(EnvInstanceID, "default-instance"),
		InstanceGroup:  EnvOr(EnvInstanceGroup, "default-group"),
		ActiveRulePath: EnvOr(EnvActiveRulePath, "./rules/active_rule.json"),
	}
}

func GetGeneralConfig() *GeneralConfig {
	generalConfigOnce.Do(func() {
		generalConfig = ReadFromEnv()
	})
	return generalConfig
}
