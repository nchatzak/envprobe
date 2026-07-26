package probe

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
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

func TestToJSONResults(t *testing.T) {
	tests := []struct {
		name  string
		input []Result
		want  []jsonResult
	}{
		{
			name: "all tools found",
			input: []Result{
				{Name: "git", Found: true, Version: "2.30.0", Path: "/opt/git", Duration: 1500 * time.Millisecond},
				{Name: "go", Found: true, Version: "1.16.0", Path: "/opt/go", Duration: 2500 * time.Millisecond},
			},
			want: []jsonResult{
				{Name: "git", Found: true, Version: "2.30.0", Path: "/opt/git", DurationMs: 1500},
				{Name: "go", Found: true, Version: "1.16.0", Path: "/opt/go", DurationMs: 2500},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toJSONResults(tt.input)
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("toJSONResults() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestRenderJSON(t *testing.T) {
	t.Run("omits empty fields for not-found tool", func(t *testing.T) {
		results := []Result{
			{Name: "nonexistenttool", Found: false, Version: "", Path: "", Duration: 0},
		}
		var builder strings.Builder
		RenderJSON(&builder, results)
		gotOutput := builder.String()

		var got []map[string]any
		if err := json.Unmarshal([]byte(gotOutput), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}

		if value, ok := got[0]["found"]; !ok || value != false {
			t.Errorf("RenderJSON() output does not contain expected found status for nonexistent tool")
		}

		if _, ok := got[0]["version"]; ok {
			t.Errorf("RenderJSON() output should not contain 'version' field for nonexistent tool")
		}

		if _, ok := got[0]["path"]; ok {
			t.Errorf("RenderJSON() output should not contain 'path' field for nonexistent tool")
		}
	})

	t.Run("includes version for found tool", func(t *testing.T) {
		results := []Result{
			{Name: "go", Found: true, Version: "1.16.0", Path: "/opt/go", Duration: 2500 * time.Millisecond},
		}
		var builder strings.Builder
		RenderJSON(&builder, results)
		gotOutput := builder.String()

		var got []map[string]any
		if err := json.Unmarshal([]byte(gotOutput), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}

		if value, ok := got[0]["found"]; !ok || value != true {
			t.Errorf("RenderJSON() output does not contain expected found status for found tool")
		}

		if value, ok := got[0]["version"]; !ok || value != "1.16.0" {
			t.Errorf("RenderJSON() output does not contain expected version for found tool")
		}

		if value, ok := got[0]["path"]; !ok || value != "/opt/go" {
			t.Errorf("RenderJSON() output does not contain expected path for found tool")
		}

		if value, ok := got[0]["duration_ms"]; !ok || value != float64(2500) {
			t.Errorf("RenderJSON() output does not contain expected duration for found tool")
		}
	})
}
