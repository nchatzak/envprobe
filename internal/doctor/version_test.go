package doctor

import (
	"testing"
)

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "Go version",
			output:   "go version go1.26.5 darwin/arm64",
			expected: "1.26.5",
		},
		{
			name:     "Version with v prefix",
			output:   "v24.14.1",
			expected: "24.14.1",
		},
		{
			name: "OpenJDK version",
			output: `openjdk version "21.0.11" 2026-04-21 LTS
OpenJDK Runtime Environment Temurin-21.0.11+10 (build 21.0.11+10-LTS)
OpenJDK 64-Bit Server VM Temurin-21.0.11+10 (build 21.0.11+10-LTS, mixed mode, sharing)`,
			expected: "21.0.11",
		},
		{
			name: "Docker version",
			output: `Client:
 Version:           29.4.0
 API version:       1.54
 Go version:        go1.26.3
 Git commit:        9d7ad9f`,
			expected: "29.4.0",
		},
		{
			name:     "Empty output",
			output:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseVersionOutput(tt.output); got != tt.expected {
				t.Errorf("parseVersionOutput() = %v, want %v", got, tt.expected)
			}
		})
	}
}
