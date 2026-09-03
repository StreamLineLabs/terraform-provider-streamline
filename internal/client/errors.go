package client

import (
	"errors"
	"fmt"
)

// NotFoundError identifies a confirmed missing remote object. Callers must use
// IsNotFound rather than matching error strings before removing Terraform
// state.
type NotFoundError struct {
	Resource   string
	Identifier string
	Cause      error
}

func (e *NotFoundError) Error() string {
	message := fmt.Sprintf("%s not found", e.Resource)
	if e.Identifier != "" {
		message += ": " + e.Identifier
	}
	if e.Cause != nil {
		message += ": " + e.Cause.Error()
	}
	return message
}

// Unwrap preserves an underlying protocol or transport error when present.
func (e *NotFoundError) Unwrap() error {
	return e.Cause
}

// NewNotFoundError creates a typed error for a confirmed missing object.
func NewNotFoundError(resource, identifier string, cause error) *NotFoundError {
	return &NotFoundError{
		Resource:   resource,
		Identifier: identifier,
		Cause:      cause,
	}
}

// IsNotFound reports whether err, including any wrapped error, is a confirmed
// missing-object result.
func IsNotFound(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}

// APIError represents an error returned by the Streamline API.
type APIError struct {
	StatusCode int
	Message    string
	RequestID  string
}

func (e *APIError) Error() string {
	if e.RequestID != "" {
		return fmt.Sprintf("streamline API error (status %d, request %s): %s",
			e.StatusCode, e.RequestID, e.Message)
	}
	return fmt.Sprintf("streamline API error (status %d): %s",
		e.StatusCode, e.Message)
}

// IsNotFound returns true if the error indicates a 404 response.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == 404
}

// IsConflict returns true if the error indicates a 409 response.
func (e *APIError) IsConflict() bool {
	return e.StatusCode == 409
}

// IsRetryable returns true if the error is transient and can be retried.
func (e *APIError) IsRetryable() bool {
	return e.StatusCode == 429 || e.StatusCode >= 500
}
