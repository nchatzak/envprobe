package cmd

import (
	"errors"
	"fmt"
	"io"
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

// checkLoader reports the checks a config file defines and the path it was read
// from. source is empty only when no file was read.
type checkLoader func() (checks []probe.Check, source string, err error)

// printConfigSource names the config file a command is working from. An empty
// path prints nothing: no file was read, so there is nothing to name.
func printConfigSource(w io.Writer, path string) {
	if path == "" {
		return
	}
	fmt.Fprintf(w, "using %s\n", path)
}

func configuredChecks() ([]probe.Check, string, error) {
	v, err := loadConfig()
	if err != nil {
		return nil, "", err
	}
	source := v.ConfigFileUsed()
	checks, err := checksFromConfig(v)
	return checks, source, err
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
		// Malformed YAML. ConfigFileUsed is set even though the read failed,
		// and the error is one line, so a prefix names the file it belongs to.
		return nil, fmt.Errorf("%s: %w", v.ConfigFileUsed(), err)
	}
	return v, nil
}

// loadConfigFile reads exactly one file, no search.
func loadConfigFile(path string) (*viper.Viper, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		// Prefixed like loadConfig's, so the same failure reads the same way
		// whether the file was searched for or named on the command line.
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return v, nil
}
