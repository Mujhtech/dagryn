package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
)

const keyPrefix = "dagryn-license-v1"

// ValidationError describes why a license key is invalid.
type ValidationError struct {
	Code    string // "malformed", "unknown_key", "bad_signature", "expired", "invalid_edition"
	Message string
}

func (e *ValidationError) Error() string { return e.Message }

// Validator verifies and decodes license keys.
type Validator struct {
	publicKeys map[string]ed25519.PublicKey
}

// NewValidator creates a Validator with the given public keyring.
func NewValidator(keys map[string]ed25519.PublicKey) *Validator {
	return &Validator{publicKeys: keys}
}

// Validate parses and verifies a license key string.
// Returns the claims if valid, or an error describing the problem.
func (v *Validator) Validate(key string) (*Claims, error) {
	const maxEncodedSegmentLen = 24 * 1024
	const maxPayloadBytes = 16 * 1024

	// 1. Split into prefix.payload.signature
	parts := strings.SplitN(key, ".", 3)
	if len(parts) != 3 || parts[0] != keyPrefix {
		return nil, &ValidationError{
			Code:    "malformed",
			Message: "license key must start with " + keyPrefix,
		}
	}

	if len(parts[1]) > maxEncodedSegmentLen || len(parts[2]) > maxEncodedSegmentLen {
		return nil, &ValidationError{Code: "malformed", Message: "license key segments too large"}
	}

	// 2. Decode payload
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, &ValidationError{Code: "malformed", Message: "invalid payload encoding"}
	}
	if len(payloadBytes) > maxPayloadBytes {
		return nil, &ValidationError{Code: "malformed", Message: "license payload too large"}
	}

	// 3. Decode signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, &ValidationError{Code: "malformed", Message: "invalid signature encoding"}
	}

	// 4. Read key ID from untrusted payload (selection only).
	var envelope struct {
		KeyID string `json:"kid"`
	}
	if err := json.Unmarshal(payloadBytes, &envelope); err != nil || envelope.KeyID == "" {
		return nil, &ValidationError{Code: "malformed", Message: "missing key id in license payload"}
	}
	pub, ok := v.publicKeys[envelope.KeyID]
	if !ok {
		return nil, &ValidationError{Code: "unknown_key", Message: "license key id is not trusted by this binary"}
	}

	// 5. Verify Ed25519 signature over raw payload bytes
	if !ed25519.Verify(pub, payloadBytes, sigBytes) {
		return nil, &ValidationError{Code: "bad_signature", Message: "license signature verification failed"}
	}

	// 6. Unmarshal claims
	var claims Claims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return nil, &ValidationError{Code: "malformed", Message: "invalid license payload"}
	}

	// 7. Validate core claims
	if claims.Issuer != "Dagryn Inc." {
		return nil, &ValidationError{Code: "malformed", Message: "invalid issuer"}
	}
	if claims.IssuedAt <= 0 || claims.ExpiresAt <= 0 || claims.IssuedAt > claims.ExpiresAt {
		return nil, &ValidationError{Code: "malformed", Message: "invalid license timestamps"}
	}
	if claims.GraceDays < 0 || claims.GraceDays > 90 {
		return nil, &ValidationError{Code: "malformed", Message: "invalid grace period"}
	}

	// 8. Validate edition
	switch claims.Edition {
	case EditionPro, EditionEnterprise:
		// ok
	default:
		return nil, &ValidationError{
			Code:    "invalid_edition",
			Message: fmt.Sprintf("unknown edition %q", claims.Edition),
		}
	}

	// 9. Reject licenses that are past hard expiry.
	if claims.IsHardExpired() {
		return nil, &ValidationError{Code: "expired", Message: "license is expired"}
	}

	return &claims, nil
}
