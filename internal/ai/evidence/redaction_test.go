package evidence

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedact_BearerToken(t *testing.T) {
	input := `Authorization: Bearer eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.payload.sig`
	result := Redact(input)
	assert.NotContains(t, result, "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedact_AWSAccessKey(t *testing.T) {
	input := "aws_access_key_id = AKIAIOSFODNN7EXAMPLE"
	result := Redact(input)
	assert.NotContains(t, result, "AKIAIOSFODNN7EXAMPLE")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedact_AWSSecretKey(t *testing.T) {
	input := "aws_secret_access_key = wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY"
	result := Redact(input)
	assert.NotContains(t, result, "wJalrXUtnFEMI")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedact_GitHubTokens(t *testing.T) {
	tests := []struct {
		name  string
		token string
	}{
		{"ghp", "ghp_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"},
		{"gho", "gho_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"},
		{"ghs", "ghs_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"},
		{"ghr", "ghr_ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghij"},
		{"pat", "github_pat_11AAAAAAAAAAAAAAAAAA_BBBBBBBBBBBBBBBBBBBBBBbbb"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := "token=" + tt.token
			result := Redact(input)
			assert.NotContains(t, result, tt.token)
			assert.Contains(t, result, "[REDACTED]")
		})
	}
}

func TestRedact_JWTToken(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U"
	result := Redact("token: " + jwt)
	assert.NotContains(t, result, "eyJhbGciOiJIUzI1NiJ9")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedact_StripeKeys(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"live_secret", "sk_live_4eC39HqLyjWDarjtT1zdp7dc"},
		{"test_secret", "sk_test_4eC39HqLyjWDarjtT1zdp7dc"},
		{"live_publish", "pk_live_4eC39HqLyjWDarjtT1zdp7dc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Redact("STRIPE_KEY=" + tt.key)
			assert.NotContains(t, result, tt.key)
			assert.Contains(t, result, "[REDACTED]")
		})
	}
}

func TestRedact_GenericKeyValuePatterns(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"api_key", "api_key=super_secret_value_123"},
		{"secret_key", "secret_key: mysecretvalue"},
		{"access_token", "access_token=tok_abc123xyz"},
		{"password", "password=hunter2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Redact(tt.input)
			assert.Contains(t, result, "[REDACTED]")
		})
	}
}

func TestRedact_ConnectionStrings(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"postgres", "DATABASE_URL=postgres://user:pass@host:5432/db"},
		{"redis", "REDIS_URL=redis://:password@host:6379/0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Redact(tt.input)
			assert.Contains(t, result, "[REDACTED]")
		})
	}
}

func TestRedact_NoSecrets(t *testing.T) {
	input := "this is a normal log line with no secrets"
	result := Redact(input)
	assert.Equal(t, input, result)
}

func TestRedact_MultipleSecrets(t *testing.T) {
	input := "api_key=secret123 and password=hunter2 in same line"
	result := Redact(input)
	assert.NotContains(t, result, "secret123")
	assert.NotContains(t, result, "hunter2")
}

func TestRedactEnvValues(t *testing.T) {
	t.Setenv("TEST_SECRET_KEY", "my-secret-value-1234")
	input := "error: failed with my-secret-value-1234 in output"
	result := RedactEnvValues(input, []string{"TEST_SECRET_KEY"})
	assert.NotContains(t, result, "my-secret-value-1234")
	assert.Contains(t, result, "[REDACTED]")
}

func TestRedactEnvValues_ShortValueSkipped(t *testing.T) {
	t.Setenv("TEST_AUTH_X", "abc")
	input := "abc should not be redacted"
	result := RedactEnvValues(input, []string{"TEST_AUTH_X"})
	assert.Contains(t, result, "abc")
}

func TestSensitiveEnvVarNames(t *testing.T) {
	t.Setenv("MY_API_KEY", "value1")
	t.Setenv("MY_SECRET", "value2")
	t.Setenv("MY_TOKEN", "value3")
	t.Setenv("SAFE_VAR", "value4")

	names := SensitiveEnvVarNames()
	found := map[string]bool{}
	for _, n := range names {
		found[n] = true
	}
	assert.True(t, found["MY_API_KEY"])
	assert.True(t, found["MY_SECRET"])
	assert.True(t, found["MY_TOKEN"])
	assert.False(t, found["SAFE_VAR"])
}

func TestRedactAll(t *testing.T) {
	t.Setenv("TEST_PASSWORD_X", "envpassword1234")
	input := "api_key=inline_secret and also envpassword1234 leaked"
	result := RedactAll(input)
	assert.NotContains(t, result, "inline_secret")
	assert.NotContains(t, result, "envpassword1234")
}
