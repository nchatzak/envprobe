package probe

import (
	"regexp"
	"strings"
)

// versionPattern matches the version-shaped run in whatever a tool printed:
// - "go version go1.26.5 darwin/arm64" -> "1.26.5"
// - "v24.14.1" -> "24.14.1"
// - "openjdk version \"21.0.11\" ..." -> "21.0.11"
var versionPattern = regexp.MustCompile(`(?i)\b(?:go|v)?\s*"?(\d+(?:\.\d+)+)`)

func parseVersionOutput(output string) string {
	if m := versionPattern.FindStringSubmatch(output); len(m) >= 2 {
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
