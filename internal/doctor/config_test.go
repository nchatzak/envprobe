package doctor

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadChecks(t *testing.T) {
	tests := []struct {
		name string
		raws []rawCheck
		want []Check
	}{
		{
			name: "empty input",
			raws: []rawCheck{},
			want: []Check{},
		},
		{
			name: "port check",
			raws: []rawCheck{{Name: "postgres", Type: "port", With: map[string]any{"target": "localhost:5432"}}},
			want: []Check{portCheck{name: "postgres", target: "localhost:5432"}},
		},
		{
			name: "binary with version",
			raws: []rawCheck{{Name: "go", Type: "binary", With: map[string]any{"version_args": []string{"version"}}}},
			want: []Check{binaryCheck{name: "go", target: "go", versionArgs: []string{"version"}}},
		},
		{
			name: "docker daemon",
			raws: []rawCheck{{Name: "docker", Type: "docker-daemon"}},
			want: []Check{dockerDaemonCheck{}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadChecks(tt.raws)
			if err != nil {
				t.Fatalf("LoadChecks() error = %v", err)
			}
			if diff := cmp.Diff(tt.want, got, cmp.AllowUnexported(portCheck{}, binaryCheck{}, dockerDaemonCheck{})); diff != "" {
				t.Errorf("LoadChecks() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestLoadChecksErrors(t *testing.T) {
	tests := []struct {
		name            string
		raws            []rawCheck
		wantErrContains []string // slice, so the multi-error case can assert on both
	}{
		{
			name:            "missing name",
			raws:            []rawCheck{{Name: "", Type: "binary"}},
			wantErrContains: []string{"name is required"},
		},
		{
			name:            "duplicate name",
			raws:            []rawCheck{{Name: "Name1", Type: "binary"}, {Name: "Name1", Type: "binary"}},
			wantErrContains: []string{"duplicate check name"},
		},
		{
			name:            "unknown type",
			raws:            []rawCheck{{Name: "Name1", Type: "anotherType"}},
			wantErrContains: []string{"unknown check type"},
		},
		{
			name:            "constructor error",
			raws:            []rawCheck{{Name: "port", Type: "port"}},
			wantErrContains: []string{"target is required"},
		},
		{
			name:            "unkown with param",
			raws:            []rawCheck{{Name: "pg", Type: "port", With: map[string]any{"trgt": "localhost:5432"}}},
			wantErrContains: []string{"has invalid keys"},
		},
		{
			name:            "two bad entries",
			raws:            []rawCheck{{Name: "", Type: "binary"}, {Name: "x", Type: "prot"}},
			wantErrContains: []string{"name is required", "unknown check type"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checks, err := LoadChecks(tt.raws)
			if err == nil {
				t.Fatalf("expected error, got null (checks=%v)", checks)
			}
			for _, want := range tt.wantErrContains {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
		})
	}
}
