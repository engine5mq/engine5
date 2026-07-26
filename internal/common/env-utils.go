package common

import (
	"fmt"
	"os"
)

func EnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func EnvIntOr(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var parsed int
		if _, err := fmt.Sscanf(v, "%d", &parsed); err == nil {
			return parsed
		}
	}
	return def
}

func EnvBoolOr(key string, def bool) bool {
	return BoolStringOrDefault(os.Getenv(key), def)

}

func envOr(key, def string) string {
	return EnvOr(key, def)
}

func envIntOr(key string, def int) int {
	return EnvIntOr(key, def)
}

func envBoolOr(key string, def bool) bool {
	return EnvBoolOr(key, def)
}
