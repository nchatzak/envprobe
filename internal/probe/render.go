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

func RenderJSON(w io.Writer, results []Result) {
	jsonResults := toJSONResults(results)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(jsonResults); err != nil {
		fmt.Fprintf(w, "Error encoding JSON: %v\n", err)
	}
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

func Render(w io.Writer, results []Result) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)

	for _, result := range results {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", status(result.Found), result.Name, result.Version, result.Duration.Round(time.Millisecond))
	}

	_ = tw.Flush()
}

func status(found bool) string {
	if found {
		return "✓"
	}
	return "✗"
}
