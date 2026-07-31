package probe

import (
	"errors"
	"fmt"

	"github.com/go-viper/mapstructure/v2"
)

type RawCheck struct {
	Name string
	Type string
	With map[string]any
}

// checkFactory builds one Check from its config entry. Every kind in the
// registry is one of these.
type checkFactory func(name string, with map[string]any) (Check, error)

var registry = map[string]checkFactory{
	"port":          newPortCheck,
	"binary":        newBinaryCheck,
	"docker-daemon": newDockerDaemonCheck,
}

func decodeWith(with map[string]any, out any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: out, ErrorUnused: true})

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
				Err:   ErrTypeRequired,
			})
			continue
		}

		constructor, ok := registry[raw.Type]
		if !ok {
			errs = append(errs, &CheckError{
				Index: i,
				Name:  raw.Name,
				Err:   fmt.Errorf("%w %q", ErrUnknownType, raw.Type),
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
