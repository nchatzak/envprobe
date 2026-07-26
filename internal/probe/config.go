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

var registry = map[string]func(name string, with map[string]any) (Check, error){
	"port":          newPortCheck,
	"binary":        newBinaryCheck,
	"docker-daemon": newDockerDaemonCheck,
}

func decodeWith(with map[string]any, out any) error {
	decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{Result: out, ErrorUnused: true})

	if err != nil {
		return fmt.Errorf("cannot create decoder, %w", err)
	}

	return decoder.Decode(with)
}

func LoadChecks(raws []RawCheck) ([]Check, error) {
	var errs []error
	seen := make(map[string]bool, len(raws))
	checks := make([]Check, 0, len(raws))

	for i, raw := range raws {
		if raw.Name == "" {
			errs = append(errs, fmt.Errorf("checks[%d]: name is required", i))
			continue
		}
		if seen[raw.Name] {
			errs = append(errs, fmt.Errorf("duplicate check name %q", raw.Name))
			continue
		}
		seen[raw.Name] = true

		constructor, ok := registry[raw.Type]
		if !ok {
			errs = append(errs, fmt.Errorf("unknown check type %q for check %q", raw.Type, raw.Name))
			continue
		}
		check, err := constructor(raw.Name, raw.With)
		if err != nil {
			errs = append(errs, fmt.Errorf("check %q: %w", raw.Name, err))
			continue
		}
		checks = append(checks, check)
	}

	if err := errors.Join(errs...); err != nil {
		return nil, err
	}

	return checks, nil
}
