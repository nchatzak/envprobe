package probe

import (
	"encoding/json"
	"fmt"
	"io"
	"text/tabwriter"
	"time"
)

type jsonResult struct {
	Name       string `json:"name"`
	Found      bool   `json:"found"`
	Path       string `json:"path,omitempty"`
	Version    string `json:"version,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

func RenderJSON(w io.Writer, results []Result) error {
	jsonResults := toJSONResults(results)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonResults)
}

func toJSONResults(results []Result) []jsonResult {
	jsonResults := make([]jsonResult, len(results))
	for i, result := range results {
		jsonResults[i] = jsonResult{
			Name:       result.Name,
			Found:      result.Found,
			Path:       result.Path,
			Version:    result.Version,
			DurationMs: result.Duration.Milliseconds(),
		}
	}
	return jsonResults
}

func Render(w io.Writer, results []Result) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	for _, result := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", status(result.Found), result.Name, result.Version, result.Duration.Round(time.Millisecond))
	}

	return tw.Flush()
}

func status(found bool) string {
	if found {
		return "✓"
	}
	return "✗"
}

// PrintSummary writes a one-line count of how many checks passed. Zero results
// write nothing.
func PrintSummary(w io.Writer, results []Result) {
	if len(results) == 0 {
		return
	}
	passed := len(results) - CountFailed(results)
	fmt.Fprintf(w, "%d of %d checks passed\n", passed, len(results))
}
