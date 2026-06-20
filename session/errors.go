package session

import (
	"errors"
	"fmt"
)

// Error codes for session operations.
// These can be used by HTTP handlers to return appropriate status codes.
const (
	// ErrCodeInvalidSession indicates the session is malformed or missing required fields.
	ErrCodeInvalidSession = "SESSION_INVALID"

	// ErrCodeSizeLimitExceeded indicates the session data exceeds the configured limit.
	ErrCodeSizeLimitExceeded = "SESSION_SIZE_EXCEEDED"

	// ErrCodeNotSerializable indicates session data cannot be serialized to JSON.
	ErrCodeNotSerializable = "SESSION_NOT_SERIALIZABLE"

	// ErrCodeNotFound indicates the session does not exist.
	ErrCodeNotFound = "SESSION_NOT_FOUND"

	// ErrCodeExpired indicates the session has expired.
	ErrCodeExpired = "SESSION_EXPIRED"

	// ErrCodeStoreClosed indicates the store has been closed.
	ErrCodeStoreClosed = "STORE_CLOSED"

	// ErrCodeSessionLimitExceeded indicates the user has too many concurrent sessions.
	ErrCodeSessionLimitExceeded = "SESSION_LIMIT_EXCEEDED"

	// ErrCodeSiteMismatch indicates the session belongs to a different site.
	ErrCodeSiteMismatch = "SESSION_SITE_MISMATCH"
)

// SessionError is a structured error with a code for programmatic handling.
type SessionError struct {
	// Code is a machine-readable error code.
	Code string

	// Message is a human-readable error message.
	Message string

	// Details contains additional error context.
	Details map[string]any

	// Cause is the underlying error, if any.
	Cause error
}

func (e *SessionError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *SessionError) Unwrap() error {
	return e.Cause
}

// Is implements errors.Is for SessionError.
func (e *SessionError) Is(target error) bool {
	// Match against sentinel errors
	switch e.Code {
	case ErrCodeNotFound:
		return errors.Is(target, ErrNotFound)
	case ErrCodeExpired:
		return errors.Is(target, ErrExpired)
	case ErrCodeInvalidSession:
		return errors.Is(target, ErrInvalidSession)
	case ErrCodeSizeLimitExceeded:
		return errors.Is(target, ErrSizeLimitExceeded)
	case ErrCodeStoreClosed:
		return errors.Is(target, ErrClosed)
	case ErrCodeSessionLimitExceeded:
		return errors.Is(target, ErrSessionLimitExceeded)
	case ErrCodeSiteMismatch:
		return errors.Is(target, ErrSiteMismatch)
	}
	return false
}

// NewSessionError creates a new SessionError.
func NewSessionError(code, message string, details map[string]any, cause error) *SessionError {
	return &SessionError{
		Code:    code,
		Message: message,
		Details: details,
		Cause:   cause,
	}
}

// ErrorCode extracts the error code from an error, if it's a SessionError.
// Returns empty string for non-SessionError errors.
func ErrorCode(err error) string {
	var se *SessionError
	if errors.As(err, &se) {
		return se.Code
	}
	return ""
}

// ErrorDetails extracts details from an error, if it's a SessionError.
func ErrorDetails(err error) map[string]any {
	var se *SessionError
	if errors.As(err, &se) {
		return se.Details
	}
	return nil
}
