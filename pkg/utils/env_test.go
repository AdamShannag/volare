package utils_test

import (
	"encoding/json"
	"github.com/AdamShannag/volare/pkg/utils"
	"os"
	"testing"
)

func TestFromEnv(t *testing.T) {
	t.Setenv("TEST_ENV_VAR", "actual_value")

	tests := []struct {
		input    string
		expected string
	}{
		{"", ""},
		{"TEST_ENV_VAR", "actual_value"},
		{"NON_EXISTENT_VAR", "NON_EXISTENT_VAR"},
	}

	for _, tt := range tests {
		got := utils.FromEnv(tt.input)
		if got != tt.expected {
			t.Errorf("FromEnv(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGetEnvJSON(t *testing.T) {
	t.Setenv("FOO", "bar")

	data, err := utils.GetEnvJSON()
	if err != nil {
		t.Fatalf("GetEnvJSON() failed: %v", err)
	}

	var parsed map[string]string
	if err = json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("GetEnvJSON output is not valid JSON: %v", err)
	}

	if val, ok := parsed["FOO"]; !ok || val != "bar" {
		t.Errorf(`GetEnvJSON() missing or incorrect value for "FOO"; got %q`, val)
	}
}

func TestLoadEnvFromJSON(t *testing.T) {
	jsonData := []byte(`{
		"ENV1": "val1",
		"ENV2": "val2"
	}`)

	if err := utils.LoadEnvFromJSON(jsonData); err != nil {
		t.Fatalf("LoadEnvFromJSON failed: %v", err)
	}

	if got := os.Getenv("ENV1"); got != "val1" {
		t.Errorf("ENV1 = %q; want %q", got, "val1")
	}
	if got := os.Getenv("ENV2"); got != "val2" {
		t.Errorf("ENV2 = %q; want %q", got, "val2")
	}
}
