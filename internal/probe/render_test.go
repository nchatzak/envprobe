package probe

import (
	"encoding/json"
	"errors"
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
		{
			name: "problems are shown",
			results: []Result{
				{Name: "git", Found: true, Version: "", Problem: problemVersionCommandFailed},
				{Name: "postgres", Found: false, Problem: problemConnectionRefused},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var builder strings.Builder
			err := Render(&builder, tt.results)
			if err != nil {
				t.Fatalf("Render returned an error: %v", err)
			}
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

				if r.Problem != "" && !strings.Contains(gotOutput, "("+r.Problem+")") {
					t.Errorf("Render() output does not contain expected problem %q", r.Problem)
				}
			}
		})
	}

	t.Run("returns the writer's error", func(t *testing.T) {
		// A non-empty slice matters: Render only writes inside the loop, so
		// with no results Flush has nothing to push and returns nil.
		err := Render(errWriter{}, []Result{{Name: "git", Found: true}})
		if !errors.Is(err, errWrite) {
			t.Errorf("Render() error = %v, want %v", err, errWrite)
		}
	})
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
		{
			name: "some tools not found/with problems",
			input: []Result{
				{Name: "git", Found: true, Version: "", Problem: problemVersionCommandFailed},
				{Name: "postgres", Found: false, Problem: problemConnectionRefused},
			},
			want: []jsonResult{
				{Name: "git", Found: true, Version: "", Problem: problemVersionCommandFailed},
				{Name: "postgres", Found: false, Problem: problemConnectionRefused},
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

func TestPrintSummary(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    string
	}{
		{
			name:    "all passed",
			results: []Result{{Name: "git", Found: true}, {Name: "go", Found: true}},
			want:    "2 of 2 checks passed\n",
		},
		{
			name:    "some failed",
			results: []Result{{Name: "git", Found: true}, {Name: "docker", Found: false}},
			want:    "1 of 2 checks passed\n",
		},
		{
			name:    "none passed",
			results: []Result{{Name: "docker", Found: false}},
			want:    "0 of 1 checks passed\n",
		},
		// doctor already warns that nothing was configured, so a "0 of 0" line
		// on top of that reports a run that did not happen.
		{
			name:    "no results",
			results: nil,
			want:    "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var builder strings.Builder
			PrintSummary(&builder, tt.results)
			if got := builder.String(); got != tt.want {
				t.Errorf("PrintSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

var errWrite = errors.New("write error")

type errWriter struct{}

func (errWriter) Write([]byte) (int, error) { return 0, errWrite }

func TestRenderJSON(t *testing.T) {
	t.Run("omits empty fields for not-found tool", func(t *testing.T) {
		results := []Result{
			{Name: "nonexistenttool", Found: false, Version: "", Path: "", Duration: 0},
		}
		var builder strings.Builder
		err := RenderJSON(&builder, results)
		if err != nil {
			t.Fatalf("RenderJSON() returned unexpected error: %v", err)
		}
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

		if _, ok := got[0]["problem"]; ok {
			t.Errorf("RenderJSON() output should not contain 'problem' field for nonexistent tool")
		}
	})

	t.Run("includes the problem when one is set", func(t *testing.T) {
		results := []Result{
			{Name: "postgres", Found: false, Problem: problemConnectionRefused},
		}
		var builder strings.Builder
		err := RenderJSON(&builder, results)
		if err != nil {
			t.Fatalf("RenderJSON() returned unexpected error: %v", err)
		}

		var got []map[string]any
		if err := json.Unmarshal([]byte(builder.String()), &got); err != nil {
			t.Fatalf("output is not valid JSON: %v", err)
		}

		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d", len(got))
		}

		if value, ok := got[0]["problem"]; !ok || value != problemConnectionRefused {
			t.Errorf("RenderJSON() problem = %v, want %q", value, problemConnectionRefused)
		}
	})

	t.Run("includes version for found tool", func(t *testing.T) {
		results := []Result{
			{Name: "go", Found: true, Version: "1.16.0", Path: "/opt/go", Duration: 2500 * time.Millisecond},
		}
		var builder strings.Builder
		err := RenderJSON(&builder, results)
		if err != nil {
			t.Fatalf("RenderJSON() returned unexpected error: %v", err)
		}
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

	t.Run("returns the writer's error", func(t *testing.T) {
		err := RenderJSON(errWriter{}, []Result{{Name: "x"}})
		if !errors.Is(err, errWrite) {
			t.Errorf("RenderJSON() returned unexpected error: %v", err)
		}
	})
}
