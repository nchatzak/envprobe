package probe

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/go-viper/mapstructure/v2"
)

type RawCheck struct {
	Name string
	Type string
	With map[string]any
}

// checkFactory builds one Check from its config entry. Every type in the
// registry is one of these.
type checkFactory func(name string, with map[string]any) (Check, error)

var registry = map[string]checkFactory{
	"port":          newPortCheck,
	"binary":        newBinaryCheck,
	"docker-daemon": newDockerDaemonCheck,
	"env":           newEnvCheck,
	"path":          newPathCheck,
}

// KnownTypes lists the registered types, sorted and comma-separated.
func KnownTypes() string {
	return strings.Join(slices.Sorted(maps.Keys(registry)), ", ")
}

// StrictKeys makes a decode fail on a key the target has no field for. Applied
// at both layers: viper's UnmarshalKey for the check itself, decodeWith for the
// with: payload, which is a map and accepts every key it is given.
func StrictKeys(dc *mapstructure.DecoderConfig) {
	dc.ErrorUnused = true
}

func decodeWith(with map[string]any, out any) error {
	dc := &mapstructure.DecoderConfig{Result: out}
	StrictKeys(dc)

	decoder, err := mapstructure.NewDecoder(dc)
	if err != nil {
		return fmt.Errorf("cannot create decoder: %w", err)
	}

	return decoder.Decode(with)
}

func LoadChecks(raws []RawCheck) ([]Check, error) {
	var errs []error
	seen := make(map[string]bool, len(raws))
	checks := make([]Check, 0, len(raws))

	for i, raw := range raws {
		if raw.Name == "" {
			errs = append(errs, &CheckError{
				Index: i,
				Err:   ErrNameRequired,
			})
			continue
		}
		if seen[raw.Name] {
			errs = append(errs, &CheckError{
				Index: i,
				Name:  raw.Name,
				Err:   ErrDuplicateName,
			})
			continue
		}
		seen[raw.Name] = true

		if raw.Type == "" {
			errs = append(errs, &CheckError{
				Index: i,
				Name:  raw.Name,
				Err:   fmt.Errorf("%w (one of: %s)", ErrCheckTypeRequired, KnownTypes()),
			})
			continue
		}

		constructor, ok := registry[raw.Type]
		if !ok {
			errs = append(errs, &CheckError{
				Index: i,
				Name:  raw.Name,
				Err:   fmt.Errorf("%w %q (one of: %s)", ErrUnknownCheckType, raw.Type, KnownTypes()),
			})
			continue
		}
		check, err := constructor(raw.Name, raw.With)
		if err != nil {
			errs = append(errs, &CheckError{
				Index: i,
				Name:  raw.Name,
				Err:   err,
			})
			continue
		}
		checks = append(checks, check)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return checks, nil
}
