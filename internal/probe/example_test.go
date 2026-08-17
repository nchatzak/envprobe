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
	// StrictKeys because the command does: without it the shipped example
	// could carry a key config init writes and doctor then refuses.
	if err := v.UnmarshalKey("checks", &raws, StrictKeys); err != nil {
		t.Fatalf("invalid checks config: %v", err)
	}

	if _, err := LoadChecks(raws); err != nil {
		t.Fatalf("example config failed to load: %v", err)
	}

	seen := make(map[string]bool, len(raws))
	for _, raw := range raws {
		seen[raw.Type] = true
	}

	for checkType := range registry {
		if !seen[checkType] {
			t.Errorf("example.yaml has no %q entry", checkType)
		}
	}
}

func TestExampleConfigListsRegisteredTypes(t *testing.T) {
	if !strings.Contains(ExampleConfig, KnownTypes()) {
		t.Errorf("example.yaml header does not list the registered types %q", KnownTypes())
	}
}
