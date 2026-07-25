package doctor

import (
	"context"
	"slices"
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

func TestSelectChecks(t *testing.T) {
	allChecks := []Check{
		fakeCheck{name: "go", result: Result{Name: "go", Found: true}},
		fakeCheck{name: "git", result: Result{Name: "git", Found: false}},
	}

	allCheckNames := make([]string, 0, len(allChecks))
	for _, c := range allChecks {
		allCheckNames = append(allCheckNames, c.Name())
	}

	tests := []struct {
		name  string
		names []string
		want  []string
	}{
		{"subset", []string{"git"}, []string{"git"}},
		{"empty returns all", nil, allCheckNames},
		{"unknown ignored", []string{"nope"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selectedChecks := SelectChecks(allChecks, tt.names)
			got := make([]string, 0, len(selectedChecks))
			for _, c := range selectedChecks {
				got = append(got, c.Name())
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("SelectChecks() returned unexpected check names: got %v, want %v", got, tt.want)
			}
		})
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
