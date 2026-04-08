package errors

import (
	"errors"
	"fmt"
)

type ErrorCode string

const (
	CodeNotFound      ErrorCode = "NOT_FOUND"
	CodeAlreadyExists ErrorCode = "ALREADY_EXISTS"
	CodeInvalidInput  ErrorCode = "INVALID_INPUT"
	CodeUnauthorized   ErrorCode = "UNAUTHORIZED"
	CodeForbidden      ErrorCode = "FORBIDDEN"
	CodeInternal       ErrorCode = "INTERNAL"
	CodeTimeout        ErrorCode = "TIMEOUT"
	CodeCancelled      ErrorCode = "CANCELLED"
	CodeNotImplemented ErrorCode = "NOT_IMPLEMENTED"
	CodeDatabase       ErrorCode = "DATABASE"
	CodeNetwork        ErrorCode = "NETWORK"
	CodeFilesystem     ErrorCode = "FILESYSTEM"
	CodeValidation     ErrorCode = "VALIDATION"
	CodeConfig         ErrorCode = "CONFIG"
)

var (
	ErrNotFound      = New(CodeNotFound, "resource not found")
	ErrAlreadyExists = New(CodeAlreadyExists, "resource already exists")
	ErrInvalidInput  = New(CodeInvalidInput, "invalid input")
	ErrUnauthorized   = New(CodeUnauthorized, "unauthorized")
	ErrForbidden      = New(CodeForbidden, "forbidden")
	ErrInternal       = New(CodeInternal, "internal error")
	ErrTimeout        = New(CodeTimeout, "operation timed out")
	ErrCancelled      = New(CodeCancelled, "operation cancelled")
	ErrNotImplemented = New(CodeNotImplemented, "not implemented")
	ErrDatabase       = New(CodeDatabase, "database error")
	ErrNetwork        = New(CodeNetwork, "network error")
	ErrFilesystem     = New(CodeFilesystem, "filesystem error")
	ErrValidation     = New(CodeValidation, "validation error")
	ErrConfig         = New(CodeConfig, "configuration error")
)

type AppError struct {
	Code    ErrorCode
	Message string
	Op      string
	Err     error
	Fields  map[string]interface{}
}

func New(code ErrorCode, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}

func (e *AppError) Error() string {
	if e.Op != "" {
		if e.Err != nil {
			return fmt.Sprintf("%s: %s: %s: %v", e.Code, e.Op, e.Message, e.Err)
		}
		return fmt.Sprintf("%s: %s: %s", e.Code, e.Op, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error {
	return e.Err
}

func (e *AppError) WithOp(op string) *AppError {
	cp := *e // create a copy to avoid mutating shared sentinels
	cp.Op = op
	return &cp
}

func (e *AppError) Wrap(err error) *AppError {
	cp := *e
	cp.Err = err
	return &cp
}

func (e *AppError) WithFields(fields map[string]interface{}) *AppError {
	cp := *e
	cp.Fields = fields
	return &cp
}

func Wrap(code ErrorCode, message string, err error) error {
	return &AppError{
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func WrapSentinel(sentinel *AppError, message string, err error) error {
	return &AppError{
		Code:    sentinel.Code,
		Message: message,
		Op:      sentinel.Op,
		Err:     sentinel,
	}
}

func WrapOp(code ErrorCode, op, message string, err error) error {
	return &AppError{
		Code:    code,
		Op:      op,
		Message: message,
		Err:     err,
	}
}

func Is(err error, target error) bool {
	return errors.Is(err, target)
}

func As(err error, target interface{}) bool {
	return errors.As(err, target)
}

func GetCode(err error) ErrorCode {
	if err == nil {
		return ""
	}
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr.Code
	}
	return CodeInternal
}

func IsNotFound(err error) bool {
	return errors.Is(err, ErrNotFound)
}

func IsAlreadyExists(err error) bool {
	return errors.Is(err, ErrAlreadyExists)
}

func IsInvalidInput(err error) bool {
	return errors.Is(err, ErrInvalidInput)
}

func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout)
}

func IsDatabase(err error) bool {
	return errors.Is(err, ErrDatabase)
}

func IsNetwork(err error) bool {
	return errors.Is(err, ErrNetwork)
}

func IsFilesystem(err error) bool {
	return errors.Is(err, ErrFilesystem)
}

func IsValidation(err error) bool {
	return errors.Is(err, ErrValidation)
}

func IsConfig(err error) bool {
	return errors.Is(err, ErrConfig)
}

type ValidationError struct {
	Field   string
	Message string
	Err     error
}

func (e *ValidationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("validation error on field '%s': %s: %v", e.Field, e.Message, e.Err)
	}
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

func NewValidationError(field, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
	}
}
