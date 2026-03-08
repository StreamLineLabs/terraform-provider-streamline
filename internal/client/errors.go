package client

import "fmt"

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
