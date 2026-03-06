//go:build !release

package licensing

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Dev key pair for local development and testing.
// NEVER use these keys in production builds.
var (
	devPrivateKey ed25519.PrivateKey
	devPublicKey  ed25519.PublicKey
)

func init() {
	// Deterministic seed for reproducible dev keys.
	seed := make([]byte, ed25519.SeedSize)
	copy(seed, []byte("dagryn-dev-license-seed-do-not-use"))
	devPrivateKey = ed25519.NewKeyFromSeed(seed)
	devPublicKey = devPrivateKey.Public().(ed25519.PublicKey)

	// Ensure non-release builds always have a compile-safe default keyring.
	raw, _ := json.Marshal(map[string]string{
		"dev": hex.EncodeToString(devPublicKey),
	})
	publicKeysJSON = string(raw)
}

// DevSign generates a signed license key using the dev private key.
// Only available in non-release builds. Used by unit tests.
func DevSign(claims *Claims) (string, error) {
	claims.KeyID = "dev"
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("marshal claims: %w", err)
	}
	signature := ed25519.Sign(devPrivateKey, payloadBytes)
	return fmt.Sprintf("%s.%s.%s",
		keyPrefix,
		base64.RawURLEncoding.EncodeToString(payloadBytes),
		base64.RawURLEncoding.EncodeToString(signature),
	), nil
}
