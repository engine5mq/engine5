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
		Host:           EnvOr("E5_HOST", "localhost"),
		EventPort:      EnvIntOr("E5_EVENT_PORT", 8080),
		ExhaustivePort: EnvIntOr("E5_EXHAUSTIVE_PORT", 8081),
		InstanceID:     EnvOr("E5_INSTANCE_ID", "default-instance"),
		InstanceGroup:  EnvOr("E5_INSTANCE_GROUP", "default-group"),
		ActiveRulePath: EnvOr("E5_ACTIVE_RULE_PATH", "./rules/active_rule.json"),
	}
}

func GetGeneralConfig() *GeneralConfig {
	generalConfigOnce.Do(func() {
		generalConfig = ReadFromEnv()
	})
	return generalConfig
}
