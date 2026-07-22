package doctor

import (
	"context"
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

type fakeCheck struct {
	name   string
	result Result
}

func (f fakeCheck) Name() string                   { return f.name }
func (f fakeCheck) Run(ctx context.Context) Result { return f.result }

func TestRunAll(t *testing.T) {
	checks := []Check{
		fakeCheck{name: "go", result: Result{Name: "go", Found: true}},
		fakeCheck{name: "nonexistenttool", result: Result{Name: "nonexistenttool", Found: false}},
	}

	results := RunAll(t.Context(), checks)

	if len(results) != len(checks) {
		t.Errorf("RunAll() returned %d results, want %d", len(results), len(checks))
	}

	for i, check := range checks {
		got := results[i]
		if got.Duration < 0 {
			t.Errorf("duration should be non-negative, got %v", results[i].Duration)
		}
		got.Duration = 0 // Reset duration for comparison
		if got != check.(fakeCheck).result {
			t.Errorf("RunAll() result for check %q = %v, want %v", check.Name(), results[i], check.(fakeCheck).result)
		}
	}
}

func TestDefaultChecks(t *testing.T) {
	checks := DefaultChecks()

	if len(checks) == 0 {
		t.Errorf("DefaultChecks() returned no checks")
	}
}
