package doctor

import (
	"fmt"
	"io"
)

func Render(w io.Writer, results []Result) {
	for _, result := range results {
		if result.Found {
			fmt.Fprintf(w, "✓ %s\n", result.Name)

			continue
		}
		fmt.Fprintf(w, "✗ %s\n", result.Name)
	}
}
