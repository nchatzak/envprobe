package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/nchatzak/envprobe/internal/probe"
	"github.com/spf13/viper"
)

// errNoConfig means no config file was found in any search location. Nothing
// was checked, so doctor reports it rather than passing on an empty run.
//
// It lives here rather than in probe/errors.go: every sentinel there describes
// a config entry LoadChecks refused to build, and probe has no idea config
// files exist. Viper, the search path and ConfigFileUsed are all cmd's.
var errNoConfig = errors.New(`no config file found (run "envprobe config init")`)

func configuredChecks() ([]probe.Check, error) {
	v, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return checksFromConfig(v)
}

func checksFromConfig(v *viper.Viper) ([]probe.Check, error) {
	var raws []probe.RawCheck
	if err := v.UnmarshalKey("checks", &raws); err != nil {
		return nil, fmt.Errorf("invalid checks config: %w", err)
	}
	return probe.LoadChecks(raws)
}

// loadConfig searches the standard locations. A missing config is errNoConfig:
// both callers treat "nothing configured" as a failure, so viper's error type
// is translated here, at the one place that knows a search happened.
func loadConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName("envprobe")
	v.AddConfigPath(".")

	// os.UserHomeDir resolves the right variable per platform; a literal
	// "$HOME" expands to nothing where that variable is not set.
	if home, err := os.UserHomeDir(); err == nil {
		v.AddConfigPath(home)
		v.AddConfigPath(filepath.Join(home, ".config", "envprobe"))
	}

	if err := v.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
			return nil, errNoConfig
		}
		return nil, err // malformed YAML — a real error, returned not exited
	}
	return v, nil
}

// loadConfigFile reads exactly one file, no search.
func loadConfigFile(path string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	return v, nil
}
