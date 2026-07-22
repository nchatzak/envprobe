package doctor

import (
	"regexp"
	"strings"
)

func parseVersionOutput(output string) string {
	// Try to find a semantic version-like pattern in the output.
	// Examples to handle:
	// - "go version go1.26.5 darwin/arm64" -> "1.26.5"
	// - "v24.14.1" -> "24.14.1"
	// - "openjdk version \"21.0.11\" ..." -> "21.0.11"
	// Regex: optional leading v or go, then numbers and dots (at least one dot)
	re := regexp.MustCompile(`(?i)(?:\b(?:go|v)?\s*"?)(\d+(?:\.\d+)+)`) // capture group 1
	if m := re.FindStringSubmatch(output); len(m) >= 2 {
		return m[1]
	}

	// Fallback: return the first non-empty trimmed line
	lines := strings.SplitSeq(output, "\n")
	for l := range lines {
		t := strings.TrimSpace(l)
		if t != "" {
			return t
		}
	}
	return ""
}
