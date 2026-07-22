package doctor

import (
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

func Render(w io.Writer, results []Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	defer tw.Flush()

	for _, result := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", status(result.Found), result.Name, result.Version, result.Duration.Round(time.Millisecond))
	}
}

func status(found bool) string {
	if found {
		return "✓"
	}
	return "✗"
}
