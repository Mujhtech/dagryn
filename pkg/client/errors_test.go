package client

import (
	"errors"
	"net"
	"net/url"
	"syscall"
	"testing"
)

func TestIsNetworkError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "regular error",
			err:      errors.New("some error"),
			expected: false,
		},
		{
			name:     "ErrOffline",
			err:      ErrOffline,
			expected: true,
		},
		{
			name: "NetworkError with offline flag",
			err: &NetworkError{
				Op:      "connect",
				URL:     "http://localhost:9000",
				Err:     errors.New("connection refused"),
				Offline: true,
			},
			expected: true,
		},
		{
			name: "NetworkError without offline flag",
			err: &NetworkError{
				Op:      "request",
				URL:     "http://localhost:9000",
				Err:     errors.New("some error"),
				Offline: false,
			},
			expected: false,
		},
		{
			name:     "connection refused syscall error",
			err:      syscall.ECONNREFUSED,
			expected: true,
		},
		{
			name:     "connection reset syscall error",
			err:      syscall.ECONNRESET,
			expected: true,
		},
		{
			name:     "timeout syscall error",
			err:      syscall.ETIMEDOUT,
			expected: true,
		},
		{
			name: "URL error wrapping connection refused",
			err: &url.Error{
				Op:  "Get",
				URL: "http://localhost:9000",
				Err: syscall.ECONNREFUSED,
			},
			expected: true,
		},
		{
			name:     "error message with connection refused",
			err:      errors.New("dial tcp 127.0.0.1:9000: connection refused"),
			expected: true,
		},
		{
			name:     "error message with timeout",
			err:      errors.New("i/o timeout"),
			expected: true,
		},
		{
			name:     "error message with no such host",
			err:      errors.New("dial tcp: lookup example.invalid: no such host"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsNetworkError(tt.err)
			if result != tt.expected {
				t.Errorf("IsNetworkError(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestAPIError_Is(t *testing.T) {
	tests := []struct {
		name     string
		apiErr   *APIError
		target   error
		expected bool
	}{
		{
			name:     "401 is ErrAuthRequired",
			apiErr:   &APIError{StatusCode: 401, Message: "unauthorized"},
			target:   ErrAuthRequired,
			expected: true,
		},
		{
			name:     "401 with expired message is ErrAuthExpired",
			apiErr:   &APIError{StatusCode: 401, Message: "token expired"},
			target:   ErrAuthExpired,
			expected: true,
		},
		{
			name:     "404 is ErrNotFound",
			apiErr:   &APIError{StatusCode: 404, Message: "not found"},
			target:   ErrNotFound,
			expected: true,
		},
		{
			name:     "500 is ErrServerError",
			apiErr:   &APIError{StatusCode: 500, Message: "internal error"},
			target:   ErrServerError,
			expected: true,
		},
		{
			name:     "503 is ErrServerError",
			apiErr:   &APIError{StatusCode: 503, Message: "service unavailable"},
			target:   ErrServerError,
			expected: true,
		},
		{
			name:     "429 is ErrRateLimited",
			apiErr:   &APIError{StatusCode: 429, Message: "too many requests"},
			target:   ErrRateLimited,
			expected: true,
		},
		{
			name:     "400 is not ErrAuthRequired",
			apiErr:   &APIError{StatusCode: 400, Message: "bad request"},
			target:   ErrAuthRequired,
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := errors.Is(tt.apiErr, tt.target)
			if result != tt.expected {
				t.Errorf("errors.Is(%v, %v) = %v, expected %v", tt.apiErr, tt.target, result, tt.expected)
			}
		})
	}
}

func TestNetworkError_Is(t *testing.T) {
	err := &NetworkError{
		Op:      "connect",
		URL:     "http://localhost:9000",
		Err:     errors.New("connection refused"),
		Offline: true,
	}

	if !errors.Is(err, ErrOffline) {
		t.Error("NetworkError with Offline=true should match ErrOffline")
	}

	err.Offline = false
	if errors.Is(err, ErrOffline) {
		t.Error("NetworkError with Offline=false should not match ErrOffline")
	}
}

func TestUserFriendlyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name: "network error",
			err: &NetworkError{
				Op:      "connect",
				URL:     "http://localhost:9000",
				Err:     errors.New("connection refused"),
				Offline: true,
			},
			contains: "Cannot connect to server",
		},
		{
			name:     "auth expired",
			err:      ErrAuthExpired,
			contains: "session has expired",
		},
		{
			name:     "auth required",
			err:      ErrAuthRequired,
			contains: "Authentication required",
		},
		{
			name:     "rate limited",
			err:      ErrRateLimited,
			contains: "Too many requests",
		},
		{
			name:     "server error",
			err:      ErrServerError,
			contains: "server encountered an error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := UserFriendlyError(tt.err)
			if tt.contains != "" && !containsSubstring(result, tt.contains) {
				t.Errorf("UserFriendlyError(%v) = %q, expected to contain %q", tt.err, result, tt.contains)
			}
		})
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "network error",
			err:      &NetworkError{Op: "connect", URL: "http://localhost", Err: errors.New("refused"), Offline: true},
			expected: true,
		},
		{
			name:     "rate limited",
			err:      &APIError{StatusCode: 429, Message: "rate limited"},
			expected: true,
		},
		{
			name:     "server error 500",
			err:      &APIError{StatusCode: 500, Message: "internal error"},
			expected: true,
		},
		{
			name:     "server error 503",
			err:      &APIError{StatusCode: 503, Message: "service unavailable"},
			expected: true,
		},
		{
			name:     "client error 400",
			err:      &APIError{StatusCode: 400, Message: "bad request"},
			expected: false,
		},
		{
			name:     "auth error 401",
			err:      &APIError{StatusCode: 401, Message: "unauthorized"},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsRetryable(tt.err)
			if result != tt.expected {
				t.Errorf("IsRetryable(%v) = %v, expected %v", tt.err, result, tt.expected)
			}
		})
	}
}

func TestWrapNetworkError(t *testing.T) {
	// Test with nil error
	result := WrapNetworkError("connect", "http://localhost", nil)
	if result != nil {
		t.Errorf("WrapNetworkError with nil error should return nil, got %v", result)
	}

	// Test with network error
	connErr := syscall.ECONNREFUSED
	result = WrapNetworkError("connect", "http://localhost:9000", connErr)

	var netErr *NetworkError
	if !errors.As(result, &netErr) {
		t.Fatal("expected NetworkError")
	}

	if netErr.Op != "connect" {
		t.Errorf("expected Op='connect', got %q", netErr.Op)
	}

	if netErr.URL != "http://localhost:9000" {
		t.Errorf("expected URL='http://localhost:9000', got %q", netErr.URL)
	}

	if !netErr.Offline {
		t.Error("expected Offline=true for connection refused")
	}
}

// containsSubstring is a simple helper since strings.Contains isn't imported
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock net.Error for testing
type mockNetError struct {
	timeout   bool
	temporary bool
}

func (e *mockNetError) Error() string   { return "mock net error" }
func (e *mockNetError) Timeout() bool   { return e.timeout }
func (e *mockNetError) Temporary() bool { return e.temporary }

var _ net.Error = (*mockNetError)(nil)

func TestIsNetworkError_NetError(t *testing.T) {
	err := &mockNetError{timeout: true, temporary: false}
	if !IsNetworkError(err) {
		t.Error("net.Error should be detected as network error")
	}
}
