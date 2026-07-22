package doctor

import (
	"testing"
)

func TestCheckTool(t *testing.T) {
	tests := []struct {
		name string
		tool string
		want bool
	}{
		{"go is on PATH", "go", true},
		{"non-existent tool is not on PATH", "nonexistenttool", false},
		{"empty string is not on PATH", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := checkTool(tt.tool); got.Found != tt.want {
				t.Errorf("checkTool(%q) = %v, want %v", tt.tool, got.Found, tt.want)
			}
		})
	}
}

func TestCheckAll(t *testing.T) {
	tools := []string{"go", "nonexistenttool"}
	results := CheckAll(tools)

	if len(results) != len(tools) {
		t.Errorf("CheckAll() returned %d results, want %d", len(results), len(tools))
	}

	for i, tool := range tools {
		if results[i].Name != tool {
			t.Errorf("CheckAll() result[%d].Name = %q, want %q", i, results[i].Name, tool)
		}
	}
}
