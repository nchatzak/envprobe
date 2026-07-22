package doctor

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
	}{
		{
			name: "all tools found",
			results: []Result{
				{Name: "git", Found: true, Version: "2.30.0"},
				{Name: "go", Found: true, Version: "1.16.0"},
			},
		},
		{
			name: "some tools not found",
			results: []Result{
				{Name: "git", Found: true, Version: "2.30.0"},
				{Name: "nonexistenttool", Found: false, Version: ""},
			},
		},
		{
			name: "no tools found",
			results: []Result{
				{Name: "nonexistenttool1", Found: false, Version: ""},
				{Name: "nonexistenttool2", Found: false, Version: ""},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var builder strings.Builder
			Render(&builder, tt.results)
			gotOutput := builder.String()

			if gotOutput == "" {
				t.Errorf("Render() output is empty, expected non-empty output")
			}

			for _, r := range tt.results {
				if !strings.Contains(gotOutput, r.Name) {
					t.Errorf("Render() output does not contain expected tool name %q", r.Name)
				}

				if r.Version != "" && !strings.Contains(gotOutput, r.Version) {
					t.Errorf("Render() output does not contain expected version %q", r.Version)
				}

				if !strings.Contains(gotOutput, status(r.Found)) {
					t.Errorf("Render() output does not contain expected status for %q", r.Name)
				}
			}
		})
	}
}
