package common

import "fmt"

func StringOrDefault(value, def string) string {
	if value != "" && value != "null" {
		return value
	}
	return def
}

func BoolStringOrDefault(v string, def bool) bool {
	if v == "" || v == "null" {
		return def
	}
	switch v {
	case "1", "true", "TRUE", "yes", "YES", "on", "ON", "enable", "ENABLE":
		return true
	case "0", "false", "FALSE", "no", "NO", "off", "OFF", "disable", "DISABLE":
		return false
	}
	return def
}

func StringToIntOrDefault(s string, def int) int {
	if s == "" || s == "null" {
		return def
	}
	var parsed int
	if _, err := fmt.Sscanf(s, "%d", &parsed); err == nil {
		return parsed
	}
	return def
}
