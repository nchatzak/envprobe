package cmd

import (
	"errors"
	"fmt"

	"github.com/nchatzak/envprobe/internal/probe"
	"github.com/spf13/viper"
)

func checksFromConfig(v *viper.Viper) ([]probe.Check, error) {
	var raws []probe.RawCheck
	if err := v.UnmarshalKey("checks", &raws); err != nil {
		return nil, fmt.Errorf("invalid checks config: %w", err)
	}
	return probe.LoadChecks(raws)
}

// loadConfig searches the standard locations. A missing config is not an
// error — it means "no checks configured", and callers decide what that means.
func loadConfig() (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigName("envprobe")
	v.AddConfigPath(".")
	v.AddConfigPath("$HOME")
	v.AddConfigPath("$HOME/.config/envprobe")

	if err := v.ReadInConfig(); err != nil {
		if _, ok := errors.AsType[viper.ConfigFileNotFoundError](err); ok {
			return v, nil // empty Viper, zero checks
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
