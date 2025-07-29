package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func FromEnv(env string) string {
	if env == "" {
		return ""
	}
	if val, ok := os.LookupEnv(env); ok {
		return val
	}
	return env
}

func GetEnvJSON() ([]byte, error) {
	envMap := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		} else {
			envMap[parts[0]] = ""
		}
	}
	return json.MarshalIndent(envMap, "", "  ")
}

func LoadEnvFromJSON(data []byte) error {
	var envMap map[string]string
	if err := json.Unmarshal(data, &envMap); err != nil {
		return err
	}

	for k, v := range envMap {
		if err := os.Setenv(k, v); err != nil {
			return fmt.Errorf("failed to set env %s: %w", k, err)
		}
	}
	return nil
}
