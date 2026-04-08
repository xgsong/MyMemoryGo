// Package errors defines domain-specific errors for the memory system.
// These errors provide clear, actionable information about failures.
package errors

import (
	"errors"
	"fmt"
)

// Sentinel errors for common failure cases.
var (
	// ErrNotFound indicates that a requested resource was not found.
	ErrNotFound = errors.New("resource not found")

	// ErrAlreadyExists indicates that a resource already exists.
	ErrAlreadyExists = errors.New("resource already exists")

	// ErrInvalidInput indicates that input validation failed.
	ErrInvalidInput = errors.New("invalid input")

	// ErrUnauthorized indicates that the operation requires authentication.
	ErrUnauthorized = errors.New("unauthorized")

	// ErrForbidden indicates that the operation is not allowed.
	ErrForbidden = errors.New("forbidden")

	// ErrInternal indicates an internal system error.
	ErrInternal = errors.New("internal error")

	// ErrTimeout indicates that an operation timed out.
	ErrTimeout = errors.New("operation timed out")

	// ErrCancelled indicates that an operation was cancelled.
	ErrCancelled = errors.New("operation cancelled")

	// ErrNotImplemented indicates that a feature is not yet implemented.
	ErrNotImplemented = errors.New("not implemented")
)

// MemoryError represents a domain-specific error with context.
type MemoryError struct {
	// Op is the operation that failed.
	Op string

	// Err is the underlying error.
	Err error

	// Message provides additional context.
	Message string

	// Fields contains additional structured data.
	Fields map[string]interface{}
}

// Error implements the error interface.
func (e *MemoryError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s: %v", e.Op, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %v", e.Op, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
func (e *MemoryError) Unwrap() error {
	return e.Err
}

// NewError creates a new MemoryError with the given parameters.
func NewError(op string, err error, message string) *MemoryError {
	return &MemoryError{
		Op:      op,
		Err:     err,
		Message: message,
	}
}

// WithFields adds structured fields to the error.
func (e *MemoryError) WithFields(fields map[string]interface{}) *MemoryError {
	e.Fields = fields
	return e
}

// Wrap wraps an error with operation context.
func Wrap(op string, err error) error {
	return &MemoryError{
		Op:  op,
		Err: err,
	}
}

// WrapWithMessage wraps an error with operation context and a custom message.
func WrapWithMessage(op string, err error, message string) error {
	return &MemoryError{
		Op:      op,
		Err:     err,
		Message: message,
	}
}

// IsNotFound checks if an error is ErrNotFound.
func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

// IsAlreadyExists checks if an error is ErrAlreadyExists.
func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

// IsInvalidInput checks if an error is ErrInvalidInput.
func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

// IsTimeout checks if an error is ErrTimeout.
func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

// ValidationError represents a validation failure with details.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NewValidationError creates a new validation error.
func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
