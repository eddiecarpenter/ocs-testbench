// Typed domain errors for the template package.
//
// All errors follow the go.md standard: typed structs with a stable
// Code field and constructor functions. Callers match on the code via
// errors.As and the ErrCode* sentinels.

package template

import "fmt"

// Error code constants — stable identifiers used in the Code field of
// every TemplateError. Code values are intentionally SCREAMING_SNAKE
// strings so they stand out in log lines and test assertions.
const (
	// ErrCodeUnresolvedPlaceholder is returned by the engine when a
	// {{TOKEN}} in the template has no entry in the resolved value
	// map.
	ErrCodeUnresolvedPlaceholder = "UNRESOLVED_PLACEHOLDER"

	// ErrCodeUnknownTemplateVersion is returned by the loader when
	// the template body carries a version number other than 1.
	ErrCodeUnknownTemplateVersion = "UNKNOWN_TEMPLATE_VERSION"

	// ErrCodeInvalidAVPNode is returned by the loader when an AVP
	// node in the template body is malformed (e.g. has both a
	// value and child AVPs set simultaneously).
	ErrCodeInvalidAVPNode = "INVALID_AVP_NODE"

	// ErrCodeUnknownAVP is returned by the engine when an AVP name
	// in the template cannot be resolved against the dictionary.
	ErrCodeUnknownAVP = "UNKNOWN_AVP"

	// ErrCodeEncodingTypeMismatch is returned by the engine when a
	// value in the template cannot be encoded into the
	// dictionary-specified Diameter data type.
	ErrCodeEncodingTypeMismatch = "ENCODING_TYPE_MISMATCH"
)

// TemplateError is the single typed error struct for all domain errors
// raised by this package. Callers distinguish failure shapes by
// inspecting the Code field after an errors.As assertion.
type TemplateError struct {
	// Code is one of the ErrCode* constants above.
	Code string
	// Detail is a human-readable description including relevant
	// field names and values.
	Detail string
}

// Error implements the error interface.
func (e *TemplateError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("template: %s", e.Code)
	}
	return fmt.Sprintf("template[%s]: %s", e.Code, e.Detail)
}

// errUnresolvedPlaceholder constructs a UNRESOLVED_PLACEHOLDER error
// for the named placeholder token.
func errUnresolvedPlaceholder(placeholder string) error {
	return &TemplateError{
		Code:   ErrCodeUnresolvedPlaceholder,
		Detail: fmt.Sprintf("placeholder %q has no entry in the value map", placeholder),
	}
}

// errUnknownTemplateVersion constructs an UNKNOWN_TEMPLATE_VERSION
// error for the supplied version number.
func errUnknownTemplateVersion(version int) error {
	return &TemplateError{
		Code:   ErrCodeUnknownTemplateVersion,
		Detail: fmt.Sprintf("unsupported version %d (only version 1 is recognised)", version),
	}
}

// errInvalidAVPNode constructs an INVALID_AVP_NODE error for the named
// AVP with the supplied reason.
func errInvalidAVPNode(name, reason string) error {
	return &TemplateError{
		Code:   ErrCodeInvalidAVPNode,
		Detail: fmt.Sprintf("AVP node %q: %s", name, reason),
	}
}

// errUnknownAVP constructs an UNKNOWN_AVP error for an AVP name that
// could not be resolved by the dictionary.
func errUnknownAVP(name string) error {
	return &TemplateError{
		Code:   ErrCodeUnknownAVP,
		Detail: fmt.Sprintf("AVP %q not found in dictionary", name),
	}
}

// errEncodingTypeMismatch constructs an ENCODING_TYPE_MISMATCH error
// when a value cannot be coerced into the AVP's dictionary-declared
// Diameter type.
func errEncodingTypeMismatch(avpName, expectedType, actualValue string) error {
	return &TemplateError{
		Code: ErrCodeEncodingTypeMismatch,
		Detail: fmt.Sprintf("AVP %q expects %s but got value %q",
			avpName, expectedType, actualValue),
	}
}
