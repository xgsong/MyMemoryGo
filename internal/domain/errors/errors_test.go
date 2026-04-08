package errors_test

import (
	stderrors "errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/xgsong/MyMemoryGo/internal/domain/errors"
)

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{"ErrNotFound", errors.ErrNotFound, "resource not found"},
		{"ErrAlreadyExists", errors.ErrAlreadyExists, "resource already exists"},
		{"ErrInvalidInput", errors.ErrInvalidInput, "invalid input"},
		{"ErrUnauthorized", errors.ErrUnauthorized, "unauthorized"},
		{"ErrForbidden", errors.ErrForbidden, "forbidden"},
		{"ErrInternal", errors.ErrInternal, "internal error"},
		{"ErrTimeout", errors.ErrTimeout, "operation timed out"},
		{"ErrCancelled", errors.ErrCancelled, "operation cancelled"},
		{"ErrNotImplemented", errors.ErrNotImplemented, "not implemented"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.msg, tt.err.Error())
		})
	}
}

func TestMemoryError_Error(t *testing.T) {
	tests := []struct {
		name     string
		op       string
		err      error
		message  string
		expected string
	}{
		{
			name:     "with message",
			op:       "CreateMemory",
			err:      errors.ErrNotFound,
			message:  "memory not found",
			expected: "CreateMemory: memory not found: resource not found",
		},
		{
			name:     "without message",
			op:       "DeleteMemory",
			err:      errors.ErrInvalidInput,
			message:  "",
			expected: "DeleteMemory: invalid input",
		},
		{
			name:     "with wrapped error",
			op:       "Store",
			err:      stderrors.New("custom error"),
			message:  "storage failed",
			expected: "Store: storage failed: custom error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			memErr := &errors.MemoryError{
				Op:      tt.op,
				Err:     tt.err,
				Message: tt.message,
			}
			assert.Equal(t, tt.expected, memErr.Error())
		})
	}
}

func TestMemoryError_Unwrap(t *testing.T) {
	originalErr := errors.ErrNotFound
	memErr := &errors.MemoryError{
		Op:  "test",
		Err: originalErr,
	}

	unwrapped := memErr.Unwrap()
	assert.Equal(t, originalErr, unwrapped)
}

func TestMemoryError_WithFields(t *testing.T) {
	memErr := &errors.MemoryError{
		Op:  "test",
		Err: errors.ErrNotFound,
	}

	fields := map[string]interface{}{
		"key": "value",
		"id":  123,
	}

	result := memErr.WithFields(fields)

	// WithFields should return a new copy, not modify the original
	assert.NotEqual(t, memErr, result) // Should return different pointer
	assert.Equal(t, fields, result.Fields)
	assert.Nil(t, memErr.Fields) // Original should remain unchanged
}

func TestNewError(t *testing.T) {
	op := "CreateMemory"
	err := errors.ErrNotFound
	message := "memory not found"

	memErr := errors.NewError(op, err, message)

	assert.Equal(t, op, memErr.Op)
	assert.Equal(t, err, memErr.Err)
	assert.Equal(t, message, memErr.Message)
}

func TestWrap(t *testing.T) {
	originalErr := errors.ErrNotFound
	wrappedErr := errors.Wrap("GetMemory", originalErr)

	require.NotNil(t, wrappedErr)

	memErr, ok := wrappedErr.(*errors.MemoryError)
	require.True(t, ok)
	assert.Equal(t, "GetMemory", memErr.Op)
	assert.Equal(t, originalErr, memErr.Err)
	assert.Empty(t, memErr.Message)
}

func TestWrapWithMessage(t *testing.T) {
	originalErr := errors.ErrNotFound
	wrappedErr := errors.WrapWithMessage("GetMemory", originalErr, "failed to retrieve memory")

	require.NotNil(t, wrappedErr)

	memErr, ok := wrappedErr.(*errors.MemoryError)
	require.True(t, ok)
	assert.Equal(t, "GetMemory", memErr.Op)
	assert.Equal(t, originalErr, memErr.Err)
	assert.Equal(t, "failed to retrieve memory", memErr.Message)
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"direct ErrNotFound", errors.ErrNotFound, true},
		{"wrapped ErrNotFound", errors.Wrap("test", errors.ErrNotFound), true},
		{"ErrAlreadyExists", errors.ErrAlreadyExists, false},
		{"nil error", nil, false},
		{"other error", stderrors.New("other"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errors.IsNotFound(tt.err))
		})
	}
}

func TestIsAlreadyExists(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"direct ErrAlreadyExists", errors.ErrAlreadyExists, true},
		{"wrapped ErrAlreadyExists", errors.Wrap("test", errors.ErrAlreadyExists), true},
		{"ErrNotFound", errors.ErrNotFound, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errors.IsAlreadyExists(tt.err))
		})
	}
}

func TestIsInvalidInput(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"direct ErrInvalidInput", errors.ErrInvalidInput, true},
		{"wrapped ErrInvalidInput", errors.Wrap("test", errors.ErrInvalidInput), true},
		{"ErrNotFound", errors.ErrNotFound, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errors.IsInvalidInput(tt.err))
		})
	}
}

func TestIsTimeout(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"direct ErrTimeout", errors.ErrTimeout, true},
		{"wrapped ErrTimeout", errors.Wrap("test", errors.ErrTimeout), true},
		{"ErrNotFound", errors.ErrNotFound, false},
		{"nil error", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, errors.IsTimeout(tt.err))
		})
	}
}

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		message  string
		expected string
	}{
		{
			name:     "standard validation error",
			field:    "path",
			message:  "cannot be empty",
			expected: "validation error on field 'path': cannot be empty",
		},
		{
			name:     "empty field",
			field:    "",
			message:  "invalid format",
			expected: "validation error on field '': invalid format",
		},
		{
			name:     "empty message",
			field:    "content",
			message:  "",
			expected: "validation error on field 'content': ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verr := errors.NewValidationError(tt.field, tt.message)
			assert.Equal(t, tt.field, verr.Field)
			assert.Equal(t, tt.message, verr.Message)
			assert.Equal(t, tt.expected, verr.Error())
		})
	}
}

func TestErrorsIs(t *testing.T) {
	// Test that errors.Is works correctly with wrapped errors
	originalErr := errors.ErrNotFound
	wrappedErr := errors.Wrap("test", originalErr)

	assert.True(t, stderrors.Is(wrappedErr, errors.ErrNotFound))
	assert.False(t, stderrors.Is(wrappedErr, errors.ErrAlreadyExists))
}

func TestErrorsAs(t *testing.T) {
	// Test that errors.As works correctly with MemoryError
	wrappedErr := errors.WrapWithMessage("test", errors.ErrNotFound, "test message")

	var memErr *errors.MemoryError
	assert.True(t, stderrors.As(wrappedErr, &memErr))
	assert.Equal(t, "test", memErr.Op)
	assert.Equal(t, "test message", memErr.Message)
}

func TestValidationError_AsError(t *testing.T) {
	verr := errors.NewValidationError("field", "message")

	// ValidationError should implement error interface
	var err error = verr
	assert.NotNil(t, err)
	assert.Contains(t, err.Error(), "field")
	assert.Contains(t, err.Error(), "message")
}

func TestMultipleWrapping(t *testing.T) {
	err1 := errors.ErrNotFound
	err2 := errors.Wrap("operation1", err1)
	err3 := errors.Wrap("operation2", err2)

	// errors.Is should work through multiple wrappings
	assert.True(t, stderrors.Is(err3, errors.ErrNotFound))
	assert.True(t, stderrors.Is(err2, errors.ErrNotFound))

	// Should not match other errors
	assert.False(t, stderrors.Is(err3, errors.ErrAlreadyExists))
}

func TestMemoryError_NilFields(t *testing.T) {
	memErr := &errors.MemoryError{
		Op:  "test",
		Err: errors.ErrNotFound,
	}

	// Fields should be nil initially
	assert.Nil(t, memErr.Fields)

	// Error() should still work
	assert.Contains(t, memErr.Error(), "test")
}

// Benchmark tests
func BenchmarkMemoryError_Error(b *testing.B) {
	memErr := &errors.MemoryError{
		Op:      "CreateMemory",
		Err:     errors.ErrNotFound,
		Message: "memory not found",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = memErr.Error()
	}
}

func BenchmarkWrap(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors.Wrap("test", errors.ErrNotFound)
	}
}

func BenchmarkIsNotFound(b *testing.B) {
	err := errors.Wrap("test", errors.ErrNotFound)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors.IsNotFound(err)
	}
}

func BenchmarkNewValidationError(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		errors.NewValidationError("field", "message")
	}
}
