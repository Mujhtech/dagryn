// Package client provides an HTTP client for the Dagryn API.
package client

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
)

// ErrOffline indicates the server is unreachable due to network issues.
var ErrOffline = errors.New("server unreachable")

// ErrAuthExpired indicates the authentication token has expired.
var ErrAuthExpired = errors.New("authentication expired")

// ErrAuthRequired indicates authentication is required but not provided.
var ErrAuthRequired = errors.New("authentication required")

// ErrNotFound indicates the requested resource was not found.
var ErrNotFound = errors.New("resource not found")

// ErrServerError indicates an internal server error.
var ErrServerError = errors.New("server error")

// ErrRateLimited indicates the client is being rate limited.
var ErrRateLimited = errors.New("rate limited")

// ErrQuotaExceeded indicates a billing quota has been exceeded.
var ErrQuotaExceeded = errors.New("quota exceeded")

// NetworkError wraps a network-related error with additional context.
type NetworkError struct {
	Op      string // Operation being performed (e.g., "connect", "request")
	URL     string // The URL being accessed
	Err     error  // The underlying error
	Offline bool   // True if this appears to be an offline/connectivity issue
}

func (e *NetworkError) Error() string {
	if e.Offline {
		return fmt.Sprintf("cannot reach server (%s): %v", e.URL, e.Err)
	}
	return fmt.Sprintf("%s failed (%s): %v", e.Op, e.URL, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// Is implements errors.Is for NetworkError.
func (e *NetworkError) Is(target error) bool {
	if e.Offline && target == ErrOffline {
		return true
	}
	return false
}

// APIError represents an error response from the API.
type APIError struct {
	StatusCode int
	Message    string
	ErrorCode  string
}

func (e *APIError) Error() string {
	if e.ErrorCode != "" {
		return fmt.Sprintf("API error %d (%s): %s", e.StatusCode, e.ErrorCode, e.Message)
	}
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Message)
}

// Is implements errors.Is for APIError.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrAuthExpired:
		return e.StatusCode == 401 && strings.Contains(strings.ToLower(e.Message), "expired")
	case ErrAuthRequired:
		return e.StatusCode == 401
	case ErrNotFound:
		return e.StatusCode == 404
	case ErrServerError:
		return e.StatusCode >= 500
	case ErrRateLimited:
		return e.StatusCode == 429
	case ErrQuotaExceeded:
		return e.StatusCode == 402
	}
	return false
}

// IsNetworkError returns true if the error is a network connectivity issue.
func IsNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Check for our custom NetworkError with offline flag
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return netErr.Offline
	}

	// Check for ErrOffline sentinel
	if errors.Is(err, ErrOffline) {
		return true
	}

	// Check for common network errors
	return isLowLevelNetworkError(err)
}

// isLowLevelNetworkError checks for low-level network errors.
func isLowLevelNetworkError(err error) bool {
	if err == nil {
		return false
	}

	// Unwrap to get the root cause
	rootErr := errors.Unwrap(err)
	if rootErr != nil {
		if isLowLevelNetworkError(rootErr) {
			return true
		}
	}

	// Check for net.Error (timeout, temporary errors)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Check for URL errors (connection refused, etc.)
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return isLowLevelNetworkError(urlErr.Err)
	}

	// Check for DNS errors
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}

	// Check for operation errors (connection refused, etc.)
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return true
	}

	// Check for specific syscall errors
	var syscallErr syscall.Errno
	if errors.As(err, &syscallErr) {
		switch syscallErr {
		case syscall.ECONNREFUSED,
			syscall.ECONNRESET,
			syscall.ECONNABORTED,
			syscall.ETIMEDOUT,
			syscall.ENETUNREACH,
			syscall.EHOSTUNREACH,
			syscall.ENETDOWN,
			syscall.EHOSTDOWN:
			return true
		}
	}

	// Check error message as fallback
	errMsg := strings.ToLower(err.Error())
	networkKeywords := []string{
		"connection refused",
		"connection reset",
		"no such host",
		"network is unreachable",
		"host is unreachable",
		"i/o timeout",
		"timeout",
		"dial tcp",
		"connect:",
		"eof",
		"broken pipe",
	}
	for _, keyword := range networkKeywords {
		if strings.Contains(errMsg, keyword) {
			return true
		}
	}

	return false
}

// IsAuthError returns true if the error is an authentication-related error.
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrAuthRequired) || errors.Is(err, ErrAuthExpired)
}

// IsQuotaExceeded returns true if the error is a quota exceeded error (HTTP 402).
func IsQuotaExceeded(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, ErrQuotaExceeded)
}

// IsRetryable returns true if the error is likely transient and can be retried.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Network errors are generally retryable
	if IsNetworkError(err) {
		return true
	}

	// Rate limiting is retryable
	if errors.Is(err, ErrRateLimited) {
		return true
	}

	// Server errors (5xx) are often retryable
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode >= 500 && apiErr.StatusCode < 600
	}

	return false
}

// WrapNetworkError wraps an error with network context if it's a network error.
func WrapNetworkError(op, urlStr string, err error) error {
	if err == nil {
		return nil
	}

	isOffline := isLowLevelNetworkError(err)
	return &NetworkError{
		Op:      op,
		URL:     urlStr,
		Err:     err,
		Offline: isOffline,
	}
}

// UserFriendlyError returns a user-friendly error message for the given error.
func UserFriendlyError(err error) string {
	if err == nil {
		return ""
	}

	// Network/offline errors
	if IsNetworkError(err) {
		var netErr *NetworkError
		if errors.As(err, &netErr) {
			host := extractHost(netErr.URL)
			if host != "" {
				return fmt.Sprintf("Cannot connect to server at %s. Please check your network connection and ensure the server is running.", host)
			}
		}
		return "Cannot connect to server. Please check your network connection and ensure the server is running."
	}

	// Auth errors
	if errors.Is(err, ErrAuthExpired) {
		return "Your session has expired. Please run 'dagryn auth login' to log in again."
	}
	if errors.Is(err, ErrAuthRequired) {
		return "Authentication required. Please run 'dagryn auth login' to log in."
	}

	// Rate limiting
	if errors.Is(err, ErrRateLimited) {
		return "Too many requests. Please wait a moment and try again."
	}

	// Quota exceeded
	if errors.Is(err, ErrQuotaExceeded) {
		return "Quota exceeded. Upgrade your plan at /billing or run 'dagryn billing portal' to manage your subscription."
	}

	// Server errors
	if errors.Is(err, ErrServerError) {
		return "The server encountered an error. Please try again later."
	}

	// Not found
	if errors.Is(err, ErrNotFound) {
		return "The requested resource was not found."
	}

	// API errors with messages
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Message
	}

	// Default: return the error message
	return err.Error()
}

// extractHost extracts the host from a URL string.
func extractHost(urlStr string) string {
	if urlStr == "" {
		return ""
	}
	u, err := url.Parse(urlStr)
	if err != nil {
		return ""
	}
	return u.Host
}

// IsInteractiveTerminal returns true if stdout is an interactive terminal.
func IsInteractiveTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}
