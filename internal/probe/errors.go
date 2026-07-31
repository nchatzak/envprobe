package probe

import (
	"errors"
	"fmt"
)

// Reasons a config entry cannot be turned into a Check. LoadChecks reports
// them wrapped in a CheckError, so callers match with errors.Is rather than on
// the message text.
var (
	// ErrNameRequired means the entry has no name. It is the one failure that
	// leaves CheckError.Name empty.
	ErrNameRequired = errors.New("name is required")
	// ErrDuplicateName means an earlier entry already claimed that name.
	ErrDuplicateName = errors.New("duplicate check name")
	// ErrTypeRequired means the entry has no type.
	ErrTypeRequired = errors.New("type is required")
	// ErrUnknownType means the type is not in the registry. It is wrapped with
	// the offending type name, which the sentinel alone cannot carry.
	ErrUnknownType = errors.New("unknown check type")
	// ErrTargetRequired means a kind that needs an explicit target did not get
	// one. Only port returns it: binary defaults target to the check name, and
	// docker-daemon takes no payload.
	ErrTargetRequired = errors.New("target is required")
)

// CheckError says where in the config the problem is; the wrapped Err says
// what the problem is. Shaped after fs.PathError: context fields plus a cause.
type CheckError struct {
	Index int    // position in the checks list, as written in the file
	Name  string // the entry's name, empty when that is what is missing
	Err   error  // the underlying reason, reachable via errors.Is
}

// Error renders as: checks[3] "pg": unknown check type "prot"
// The quoted name is dropped when there is none to show.
func (e *CheckError) Error() string {
	if e.Name == "" {
		return fmt.Sprintf("checks[%d]: %v", e.Index, e.Err)
	}
	return fmt.Sprintf("checks[%d] %q: %v", e.Index, e.Name, e.Err)
}

// Unwrap exposes the reason, so errors.Is reaches the sentinel underneath.
func (e *CheckError) Unwrap() error {
	return e.Err
}
