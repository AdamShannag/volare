package utils

import "os"

func FromEnv(env string) string {
	if env == "" {
		return ""
	}
	if val, ok := os.LookupEnv(env); ok {
		return val
	}
	return env
}
