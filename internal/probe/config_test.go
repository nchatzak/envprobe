package probe

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLoadChecks(t *testing.T) {
	tests := []struct {
		name string
		raws []RawCheck
		want []Check
	}{
		{
			name: "empty input",
			raws: []RawCheck{},
			want: []Check{},
		},
		{
			name: "port check",
			raws: []RawCheck{{Name: "postgres", Type: "port", With: map[string]any{"target": "localhost:5432"}}},
			want: []Check{portCheck{name: "postgres", target: "localhost:5432"}},
		},
		{
			name: "binary with version",
			raws: []RawCheck{{Name: "go", Type: "binary", With: map[string]any{"version_args": []string{"version"}}}},
			want: []Check{binaryCheck{name: "go", target: "go", versionArgs: []string{"version"}}},
		},
		{
			name: "docker daemon",
			raws: []RawCheck{{Name: "docker", Type: "docker-daemon"}},
			want: []Check{dockerDaemonCheck{name: "docker"}},
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
		raws            []RawCheck
		wantErrContains []string // slice, so the multi-error case can assert on both
	}{
		{
			name:            "missing name",
			raws:            []RawCheck{{Name: "", Type: "binary"}},
			wantErrContains: []string{"name is required"},
		},
		{
			// The index is what tells the user which of the two entries to delete.
			name:            "duplicate name",
			raws:            []RawCheck{{Name: "Name1", Type: "binary"}, {Name: "Name1", Type: "binary"}},
			wantErrContains: []string{"checks[1]", "duplicate check name"},
		},
		{
			name:            "missing type",
			raws:            []RawCheck{{Name: "Name1"}},
			wantErrContains: []string{"checks[0]", "type is required"},
		},
		{
			name:            "unknown type",
			raws:            []RawCheck{{Name: "Name1", Type: "anotherType"}},
			wantErrContains: []string{"unknown check type"},
		},
		{
			name:            "constructor error",
			raws:            []RawCheck{{Name: "port", Type: "port"}},
			wantErrContains: []string{"target is required"},
		},
		{
			name:            "unkown with param",
			raws:            []RawCheck{{Name: "pg", Type: "port", With: map[string]any{"trgt": "localhost:5432"}}},
			wantErrContains: []string{"has invalid keys"},
		},
		{
			// docker-daemon takes no payload at all, so *any* key is invalid.
			name:            "unknown with param on payload-less kind",
			raws:            []RawCheck{{Name: "docker", Type: "docker-daemon", With: map[string]any{"targt": "x"}}},
			wantErrContains: []string{"has invalid keys", "targt"},
		},
		{
			name:            "two bad entries",
			raws:            []RawCheck{{Name: "", Type: "binary"}, {Name: "x", Type: "prot"}},
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
