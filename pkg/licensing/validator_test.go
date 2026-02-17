package licensing

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validClaims() *Claims {
	return &Claims{
		LicenseID: "lic_test_001",
		KeyID:     "dev",
		Issuer:    "Dagryn Inc.",
		Subject:   "Test Corp",
		Email:     "test@example.com",
		Edition:   EditionEnterprise,
		Seats:     50,
		Features: []Feature{
			FeatureContainerExecution,
			FeaturePriorityQueue,
			FeatureSSO,
			FeatureAuditLogs,
			FeatureDashboardFull,
		},
		Limits: Limits{
			MaxProjects:       nil, // unlimited
			MaxTeamMembers:    nil,
			MaxConcurrentRuns: nil,
		},
		IssuedAt:  time.Now().Add(-24 * time.Hour).Unix(),
		ExpiresAt: time.Now().Add(365 * 24 * time.Hour).Unix(),
		GraceDays: 14,
	}
}

func TestValidateValidKey(t *testing.T) {
	claims := validClaims()
	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, err := ParsePublicKeys()
	require.NoError(t, err)

	validator := NewValidator(keys)
	result, err := validator.Validate(key)
	require.NoError(t, err)

	assert.Equal(t, "lic_test_001", result.LicenseID)
	assert.Equal(t, EditionEnterprise, result.Edition)
	assert.Equal(t, "Test Corp", result.Subject)
	assert.Equal(t, 50, result.Seats)
	assert.Len(t, result.Features, 5)
	assert.Equal(t, 14, result.GraceDays)
}

func TestValidateProEdition(t *testing.T) {
	claims := validClaims()
	claims.Edition = EditionPro
	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	result, err := validator.Validate(key)
	require.NoError(t, err)
	assert.Equal(t, EditionPro, result.Edition)
}

func TestValidateMalformedKey(t *testing.T) {
	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)

	tests := []struct {
		name string
		key  string
		code string
	}{
		{"empty", "", "malformed"},
		{"no dots", "some-random-string", "malformed"},
		{"wrong prefix", "wrong-prefix.abc.def", "malformed"},
		{"only prefix", "dagryn-license-v1.", "malformed"},
		{"two parts", "dagryn-license-v1.abc", "malformed"},
		{"bad base64 payload", "dagryn-license-v1.!!!invalid!!!.def", "malformed"},
		{"bad base64 signature", "dagryn-license-v1." + base64.RawURLEncoding.EncodeToString([]byte(`{"kid":"dev"}`)) + ".!!!bad!!!", "malformed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := validator.Validate(tt.key)
			require.Error(t, err)
			var ve *ValidationError
			require.ErrorAs(t, err, &ve)
			assert.Equal(t, tt.code, ve.Code)
		})
	}
}

func TestValidateUnknownKeyID(t *testing.T) {
	claims := validClaims()
	claims.KeyID = "unknown_kid"

	// Build a fake key with the unknown kid
	payload, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	sigB64 := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
	key := keyPrefix + "." + payloadB64 + "." + sigB64

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err := validator.Validate(key)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "unknown_key", ve.Code)
}

func TestValidateBadSignature(t *testing.T) {
	claims := validClaims()
	key, err := DevSign(claims)
	require.NoError(t, err)

	// Tamper with the payload (change one character)
	parts := strings.SplitN(key, ".", 3)
	payload, _ := base64.RawURLEncoding.DecodeString(parts[1])
	payload[0] ^= 0xFF // flip bits
	parts[1] = base64.RawURLEncoding.EncodeToString(payload)
	tampered := strings.Join(parts, ".")

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err = validator.Validate(tampered)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	// Could be "bad_signature" or "malformed" depending on what the tamper breaks
	assert.Contains(t, []string{"bad_signature", "malformed"}, ve.Code)
}

func TestValidateExpiredKey(t *testing.T) {
	claims := validClaims()
	claims.IssuedAt = time.Now().Add(-365 * 24 * time.Hour).Unix()
	claims.ExpiresAt = time.Now().Add(-30 * 24 * time.Hour).Unix() // expired 30 days ago
	claims.GraceDays = 14                                          // grace ended 16 days ago

	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err = validator.Validate(key)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "expired", ve.Code)
}

func TestValidateInGracePeriod(t *testing.T) {
	claims := validClaims()
	claims.IssuedAt = time.Now().Add(-365 * 24 * time.Hour).Unix()
	claims.ExpiresAt = time.Now().Add(-3 * 24 * time.Hour).Unix() // expired 3 days ago
	claims.GraceDays = 14                                         // still in grace (11 days left)

	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	result, err := validator.Validate(key)
	require.NoError(t, err) // should succeed (in grace)

	assert.True(t, result.IsExpired())
	assert.True(t, result.InGracePeriod())
	assert.False(t, result.IsHardExpired())
}

func TestValidateInvalidEdition(t *testing.T) {
	claims := validClaims()
	claims.Edition = "unknown"

	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err = validator.Validate(key)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "invalid_edition", ve.Code)
}

func TestValidateInvalidIssuer(t *testing.T) {
	claims := validClaims()
	claims.Issuer = "Some Other Issuer"

	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err = validator.Validate(key)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "malformed", ve.Code)
}

func TestValidateInvalidTimestamps(t *testing.T) {
	claims := validClaims()
	claims.IssuedAt = 0

	key, err := DevSign(claims)
	require.NoError(t, err)

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err = validator.Validate(key)
	require.Error(t, err)
}

func TestValidateOversizedPayload(t *testing.T) {
	// Create a key with an oversized encoded segment
	bigPayload := strings.Repeat("A", 25*1024)
	key := keyPrefix + "." + bigPayload + ".sig"

	keys, _ := ParsePublicKeys()
	validator := NewValidator(keys)
	_, err := validator.Validate(key)
	require.Error(t, err)
	var ve *ValidationError
	require.ErrorAs(t, err, &ve)
	assert.Equal(t, "malformed", ve.Code)
}

func TestParsePublicKeys(t *testing.T) {
	keys, err := ParsePublicKeys()
	require.NoError(t, err)
	assert.NotEmpty(t, keys)
	assert.Contains(t, keys, "dev")
}
