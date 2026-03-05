package scim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFilter_ValidExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		attr     string
		operator string
		value    string
	}{
		{"userName eq", `userName eq "john@example.com"`, "userName", "eq", "john@example.com"},
		{"externalId eq", `externalId eq "ext-123"`, "externalId", "eq", "ext-123"},
		{"displayName eq", `displayName eq "John Doe"`, "displayName", "eq", "John Doe"},
		{"emails.value eq", `emails.value eq "test@test.com"`, "emails.value", "eq", "test@test.com"},
		{"case insensitive op", `userName EQ "john"`, "userName", "eq", "john"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := ParseFilter(tt.input)
			require.NoError(t, err)
			require.NotNil(t, f)
			assert.Equal(t, tt.attr, f.Attribute)
			assert.Equal(t, tt.operator, f.Operator)
			assert.Equal(t, tt.value, f.Value)
		})
	}
}

func TestParseFilter_EmptyString(t *testing.T) {
	f, err := ParseFilter("")
	assert.NoError(t, err)
	assert.Nil(t, f)
}

func TestParseFilter_InvalidExpressions(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{"missing parts", "userName"},
		{"two parts only", "userName eq"},
		{"unsupported operator", `userName ne "john"`},
		{"unsupported attribute", `firstName eq "John"`},
		{"empty value", `userName eq ""`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseFilter(tt.input)
			assert.Error(t, err)
		})
	}
}
