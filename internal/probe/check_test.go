package probe

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
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

// The defaults are asserted field by field, not just counted. A previous
// regression dropped target from every binary check and TestDefaultChecks
// stayed green because it only checked the slice was non-empty.
func TestDefaultChecks(t *testing.T) {
	want := []Check{
		binaryCheck{name: "Git", target: "git", versionArgs: []string{"--version"}},
		binaryCheck{name: "Java", target: "java", versionArgs: []string{"-version"}},
		binaryCheck{name: "Go", target: "go", versionArgs: []string{"version"}},
		dockerDaemonCheck{name: "docker-daemon"},
	}

	// cmp.Diff, not slices.Equal: binaryCheck holds a []string, so it is not
	// comparable, and == on an interface wrapping it panics at runtime.
	// AllowUnexported is required because every field of a check is unexported.
	got := DefaultChecks()
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(binaryCheck{}, dockerDaemonCheck{})); diff != "" {
		t.Errorf("DefaultChecks() mismatch (-want +got):\n%s", diff)
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
