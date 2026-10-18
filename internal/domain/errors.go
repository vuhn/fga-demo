// Package domain holds entities and sentinel errors. It imports only the
// standard library.
package domain

import (
	"errors"
	"fmt"
)

// Sentinel errors. The HTTP adapter maps each to a status and error code.
var (
	ErrValidation              = errors.New("validation")
	ErrAffiliationRoleMismatch = errors.New("affiliation_role_mismatch")
	ErrNotAMember              = errors.New("not_a_member")
	ErrCannotUpload            = errors.New("cannot_upload")
	ErrUnauthenticated         = errors.New("unauthenticated")
	ErrForbidden               = errors.New("forbidden")
	ErrNotFound                = errors.New("not_found")
	ErrConflict                = errors.New("conflict")
	ErrTupleWriteFailed        = errors.New("tuple_write_failed")
)

// CheckError records which OpenFGA Check denied the action. It unwraps to
// ErrForbidden or ErrNotFound.
type CheckError struct {
	Err      error
	Relation string
	Object   string
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("%s: %s on %s", e.Err, e.Relation, e.Object)
}

func (e *CheckError) Unwrap() error { return e.Err }

// Invalid wraps ErrValidation with a human-readable reason.
func Invalid(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrValidation, fmt.Sprintf(format, args...))
}
