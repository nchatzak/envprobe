package probe

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
)

func TestExampleConfig(t *testing.T) {
	v := viper.New()
	v.SetConfigType("yaml")
	err := v.ReadConfig(strings.NewReader(ExampleConfig))
	if err != nil {
		t.Fatalf("cannot read config: %v", err)
	}

	var raws []RawCheck
	if err := v.UnmarshalKey("checks", &raws); err != nil {
		t.Fatalf("invalid checks config: %v", err)
	}

	if _, err := LoadChecks(raws); err != nil {
		t.Fatalf("example config failed to load: %v", err)
	}

	seen := make(map[string]bool, len(raws))
	for _, raw := range raws {
		seen[raw.Type] = true
	}

	for kind := range registry {
		if !seen[kind] {
			t.Errorf("example.yaml has no %q entry", kind)
		}
	}
}
