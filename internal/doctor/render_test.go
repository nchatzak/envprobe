package doctor

import (
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		want    string
	}{
		{
			name: "all tools found",
			results: []Result{
				{Name: "git", Found: true},
				{Name: "go", Found: true},
			},
			want: "✓ git\n✓ go\n",
		},
		{
			name: "some tools not found",
			results: []Result{
				{Name: "git", Found: true},
				{Name: "nonexistenttool", Found: false},
			},
			want: "✓ git\n✗ nonexistenttool\n",
		},
		{
			name: "no tools found",
			results: []Result{
				{Name: "nonexistenttool1", Found: false},
				{Name: "nonexistenttool2", Found: false},
			},
			want: "✗ nonexistenttool1\n✗ nonexistenttool2\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var builder strings.Builder
			Render(&builder, tt.results)
			gotOutput := builder.String()
			if gotOutput != tt.want {
				t.Errorf("Render() output = %q, want %q", gotOutput, tt.want)
			}
		})
	}
}
