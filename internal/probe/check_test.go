package probe

import (
	"context"
	"testing"
)

type fakeCheck struct {
	result Result
}

func (f fakeCheck) Run(ctx context.Context) Result { return f.result }

func TestRunAll(t *testing.T) {
	checks := []Check{
		fakeCheck{result: Result{Name: "go", Found: true}},
		fakeCheck{result: Result{Name: "nonexistenttool", Found: false}},
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
			t.Errorf("RunAll() result for check %q = %v, want %v", results[i].Name, results[i], check.(fakeCheck).result)
		}
	}
}

func TestCountFailed(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    int
	}{
		{"no results", []Result{}, 0},
		{"all found", []Result{{Name: "go", Found: true}, {Name: "git", Found: true}}, 0},
		{"one not found", []Result{{Name: "go", Found: true}, {Name: "git", Found: false}}, 1},
		{"all not found", []Result{{Name: "go", Found: false}, {Name: "git", Found: false}}, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CountFailed(tt.results); got != tt.want {
				t.Errorf("CountFailed() = %v, want %v", got, tt.want)
			}
		})
	}
}
