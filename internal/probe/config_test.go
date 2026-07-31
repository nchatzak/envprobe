package probe

import (
	"errors"
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
		name string
		raws []RawCheck
		// Two columns because only errors we own can be matched by identity.
		// mapstructure's decode failures have no sentinel to compare against,
		// so those rows fall back to their text. Both are slices: a single
		// entry can produce more than one thing worth asserting.
		wantErrContains []string
		wantErrs        []error
	}{
		{
			name:     "missing name",
			raws:     []RawCheck{{Name: "", Type: "binary"}},
			wantErrs: []error{ErrNameRequired},
		},
		{
			// The index is what tells the user which of the two entries to delete.
			name:     "duplicate name",
			raws:     []RawCheck{{Name: "Name1", Type: "binary"}, {Name: "Name1", Type: "binary"}},
			wantErrs: []error{ErrDuplicateName},
		},
		{
			name:     "missing type",
			raws:     []RawCheck{{Name: "Name1"}},
			wantErrs: []error{ErrTypeRequired},
		},
		{
			name:     "unknown type",
			raws:     []RawCheck{{Name: "Name1", Type: "anotherType"}},
			wantErrs: []error{ErrUnknownType},
		},
		{
			name:     "port without target",
			raws:     []RawCheck{{Name: "port", Type: "port"}},
			wantErrs: []error{ErrTargetRequired},
		},
		{
			name:            "unknown with param",
			raws:            []RawCheck{{Name: "pg", Type: "port", With: map[string]any{"trgt": "localhost:5432"}}},
			wantErrContains: []string{"has invalid keys"},
		},
		{
			// No WeaklyTypedInput, so an int where a string belongs is a decode error.
			name:            "binary with wrong-typed target",
			raws:            []RawCheck{{Name: "go", Type: "binary", With: map[string]any{"target": 123}}},
			wantErrContains: []string{"decode binary config"},
		},
		{
			// docker-daemon takes no payload at all, so *any* key is invalid.
			name:            "unknown with param on payload-less kind",
			raws:            []RawCheck{{Name: "docker", Type: "docker-daemon", With: map[string]any{"targt": "x"}}},
			wantErrContains: []string{"has invalid keys", "targt"},
		},
		{
			name:     "two bad entries",
			raws:     []RawCheck{{Name: "", Type: "binary"}, {Name: "x", Type: "prot"}},
			wantErrs: []error{ErrNameRequired, ErrUnknownType},
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
			for _, want := range tt.wantErrs {
				if !errors.Is(err, want) {
					t.Errorf("error %v does not wrap %v", err, want)
				}
			}
		})
	}
}

func TestLoadChecksErrorFields(t *testing.T) {
	raw := []RawCheck{{Name: "ok", Type: "binary"}, {Name: "pg", Type: "prot"}}
	_, err := LoadChecks(raw)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	ce, ok := errors.AsType[*CheckError](err)
	if !ok {
		t.Fatalf("error type was not a *CheckError")
	}
	if ce.Name != "pg" {
		t.Errorf("error name was %q, expected %q", ce.Name, "pg")
	}
	if ce.Index != 1 {
		t.Errorf("error index was %d, expected %d", ce.Index, 1)
	}
}
