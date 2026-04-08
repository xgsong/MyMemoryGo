package errors_test

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/pkg/errors"
)

func TestSentSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		code errors.ErrorCode
		msg  string
	}{
		{"ErrNotFound", errors.ErrNotFound, errors.CodeNotFound, "resource not found"},
		{"ErrAlreadyExists", errors.ErrAlreadyExists, errors.CodeAlreadyExists, "resource already exists"},
		{"ErrInvalidInput", errors.ErrInvalidInput, errors.CodeInvalidInput, "invalid input"},
		{"ErrUnauthorized", errors.ErrUnauthorized, errors.CodeUnauthorized, "unauthorized"},
		{"ErrForbidden", errors.ErrForbidden, errors.CodeForbidden, "forbidden"},
		{"ErrInternal", errors.ErrInternal, errors.CodeInternal, "internal error"},
		{"ErrTimeout", errors.ErrTimeout, errors.CodeTimeout, "operation timed out"},
		{"ErrCancelled", errors.ErrCancelled, errors.CodeCancelled, "operation cancelled"},
		{"ErrNotImplemented", errors.ErrNotImplemented, errors.CodeNotImplemented, "not implemented"},
		{"ErrDatabase", errors.ErrDatabase, errors.CodeDatabase, "database error"},
		{"ErrNetwork", errors.ErrNetwork, errors.CodeNetwork, "network error"},
		{"ErrFilesystem", errors.ErrFilesystem, errors.CodeFilesystem, "filesystem error"},
		{"ErrValidation", errors.ErrValidation, errors.CodeValidation, "validation error"},
		{"ErrConfig", errors.ErrConfig, errors.CodeConfig, "configuration error"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			appErr, ok := tt.err.(*errors.AppError)
			require.True(t, ok)
			assert.Equal(t, tt.code, appErr.Code)
			assert.Equal(t, tt.msg, appErr.Message)
		})
	}
}

func TestAppError_Error(t *testing.T) {
	tests := []struct {
		name     string
		err      *errors.AppError
		expected string
	}{
		{
			name: "with op and wrapped error",
			err: errors.New(errors.CodeDatabase, "query failed").
				WithOp("Store").Wrap(stderrors.New("connection closed")),
			expected: "DATABASE: Store: query failed: connection closed",
		},
		{
			name:     "with op only",
			err:      errors.New(errors.CodeNotFound, "memory not found").WithOp("Get"),
			expected: "NOT_FOUND: Get: memory not found",
		},
		{
			name:     "with wrapped error only",
			err:      errors.New(errors.CodeNetwork, "request failed").Wrap(stderrors.New("timeout")),
			expected: "NETWORK: request failed: timeout",
		},
		{
			name:     "minimal error",
			err:      errors.New(errors.CodeInternal, "unexpected error"),
			expected: "INTERNAL: unexpected error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestAppError_Unwrap(t *testing.T) {
	originalErr := stderrors.New("original error")
	appErr := errors.New(errors.CodeInternal, "wrapper").Wrap(originalErr)

	unwrapped := appErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestAppError_WithOp(t *testing.T) {
	err := errors.New(errors.CodeDatabase, "query failed")
	result := err.WithOp("Store")

	assert.Equal(t, err, result)
	assert.Equal(t, "Store", err.Op)
}

func TestAppError_Wrap(t *testing.T) {
	originalErr := stderrors.New("original error")
	err := errors.New(errors.CodeInternal, "wrapper")
	result := err.Wrap(originalErr)

	assert.Equal(t, err, result)
	assert.Equal(t, originalErr, err.Err)
}

func TestAppError_WithFields(t *testing.T) {
	err := errors.New(errors.CodeDatabase, "query failed")
	fields := map[string]interface{}{
		"query": "SELECT * FROM memories",
		"table": "memories",
	}
	result := err.WithFields(fields)

	assert.Equal(t, err, result)
	assert.Equal(t, fields, err.Fields)
}

func TestWrap(t *testing.T) {
	originalErr := stderrors.New("original error")
	wrappedErr := errors.Wrap(errors.CodeDatabase, "query failed", originalErr)

	require.NotNil(t, wrappedErr)
	appErr, ok := wrappedErr.(*errors.AppError)
	require.True(t, ok)
	assert.Equal(t, errors.CodeDatabase, appErr.Code)
	assert.Equal(t, "query failed", appErr.Message)
	assert.Equal(t, originalErr, appErr.Err)
}

func TestWrapOp(t *testing.T) {
	originalErr := stderrors.New("original error")
	wrappedErr := errors.WrapOp(errors.CodeDatabase, "Store", "query failed", originalErr)

	require.NotNil(t, wrappedErr)
	appErr, ok := wrappedErr.(*errors.AppError)
	require.True(t, ok)
	assert.Equal(t, errors.CodeDatabase, appErr.Code)
	assert.Equal(t, "Store", appErr.Op)
	assert.Equal(t, "query failed", appErr.Message)
	assert.Equal(t, originalErr, appErr.Err)
}

func TestGetCode(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected errors.ErrorCode
	}{
		{"nil error", nil, ""},
		{"AppError", errors.New(errors.CodeDatabase, "test"), errors.CodeDatabase},
		{"wrapped AppError", errors.Wrap(errors.CodeNetwork, "test", stderrors.New("inner")), errors.CodeNetwork},
		{"standard error", stderrors.New("standard"), errors.CodeInternal},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errors.GetCode(tt.err))
		})
	}
}

func TestIsFunctions(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		checker  func(error) bool
		expected bool
	}{
		{"IsNotFound - direct", errors.ErrNotFound, errors.IsNotFound, true},
		{"IsNotFound - wrapped", errors.WrapSentinel(errors.ErrNotFound, "test", stderrors.New("inner")), errors.IsNotFound, true},
		{"IsNotFound - different", errors.ErrDatabase, errors.IsNotFound, false},
		{"IsDatabase - direct", errors.ErrDatabase, errors.IsDatabase, true},
		{"IsDatabase - wrapped", errors.WrapSentinel(errors.ErrDatabase, "test", stderrors.New("inner")), errors.IsDatabase, true},
		{"IsDatabase - different", errors.ErrNotFound, errors.IsDatabase, false},
		{"IsNetwork - direct", errors.ErrNetwork, errors.IsNetwork, true},
		{"IsFilesystem - direct", errors.ErrFilesystem, errors.IsFilesystem, true},
		{"IsValidation - direct", errors.ErrValidation, errors.IsValidation, true},
		{"IsConfig - direct", errors.ErrConfig, errors.IsConfig, true},
		{"IsTimeout - direct", errors.ErrTimeout, errors.IsTimeout, true},
		{"IsAlreadyExists - direct", errors.ErrAlreadyExists, errors.IsAlreadyExists, true},
		{"IsInvalidInput - direct", errors.ErrInvalidInput, errors.IsInvalidInput, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.checker(tt.err))
		})
	}
}

func TestValidationError(t *testing.T) {
	verr := errors.NewValidationError("path", "cannot be empty")

	assert.Equal(t, "path", verr.Field)
	assert.Equal(t, "cannot be empty", verr.Message)
	assert.Equal(t, "validation error on field 'path': cannot be empty", verr.Error())
}

func TestErrorsIs(t *testing.T) {
	originalErr := errors.ErrNotFound
	wrappedErr := errors.Wrap(errors.CodeNotFound, "test", originalErr)

	assert.True(t, stderrors.Is(wrappedErr, errors.ErrNotFound))
	assert.False(t, stderrors.Is(wrappedErr, errors.ErrDatabase))
}

func TestErrorsAs(t *testing.T) {
	wrappedErr := errors.WrapOp(errors.CodeDatabase, "Store", "query failed", stderrors.New("inner"))

	var appErr *errors.AppError
	assert.True(t, stderrors.As(wrappedErr, &appErr))
	assert.Equal(t, errors.CodeDatabase, appErr.Code)
	assert.Equal(t, "Store", appErr.Op)
}

func TestMultipleWrapping(t *testing.T) {
	err1 := errors.ErrNotFound
	err2 := errors.Wrap(errors.CodeNotFound, "layer1", err1)
	err3 := errors.Wrap(errors.CodeNotFound, "layer2", err2)

	assert.True(t, stderrors.Is(err3, errors.ErrNotFound))
	assert.True(t, stderrors.Is(err2, errors.ErrNotFound))
	assert.False(t, stderrors.Is(err3, errors.ErrDatabase))
}

func TestChaining(t *testing.T) {
	err := errors.New(errors.CodeDatabase, "query failed").
		WithOp("Store").
		Wrap(stderrors.New("connection closed")).
		WithFields(map[string]interface{}{"query": "SELECT *"})

	assert.Equal(t, errors.CodeDatabase, err.Code)
	assert.Equal(t, "Store", err.Op)
	assert.NotNil(t, err.Err)
	assert.NotNil(t, err.Fields)
}

func BenchmarkAppError_Error(b *testing.B) {
	err := errors.New(errors.CodeDatabase, "query failed").
		WithOp("Store").
		Wrap(stderrors.New("connection closed"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err.Error()
	}
}

func BenchmarkWrap(b *testing.B) {
	innerErr := stderrors.New("inner error")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors.Wrap(errors.CodeDatabase, "query failed", innerErr)
	}
}

func BenchmarkIsNotFound(b *testing.B) {
	err := errors.Wrap(errors.CodeNotFound, "test", errors.ErrNotFound)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors.IsNotFound(err)
	}
}
