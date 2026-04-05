package licensing

import (
	"crypto/ed25519"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// license_keys.json contains a JSON map of key ID -> hex-encoded Ed25519 public key.
// Example: {"kp_2026q1":"<hex>","kp_2025q4":"<hex>"}
//
// To update: fetch from the license server and write to license_keys.json:
//
//	make fetch-license-keys
//
//go:embed license_keys.json
var publicKeysJSON string

// ParsePublicKeys decodes the embedded keyring.
// Returns an empty map (not an error) when no keys are embedded,
// allowing the caller to decide whether that is fatal.
func ParsePublicKeys() (map[string]ed25519.PublicKey, error) {
	var raw map[string]string
	if err := json.Unmarshal([]byte(publicKeysJSON), &raw); err != nil {
		return nil, fmt.Errorf("license: invalid keyring JSON embedded in binary: %w", err)
	}

	out := make(map[string]ed25519.PublicKey, len(raw))
	for kid, hexKey := range raw {
		key, err := hex.DecodeString(hexKey)
		if err != nil {
			return nil, fmt.Errorf("license: invalid public key hex for %q: %w", kid, err)
		}
		if len(key) != ed25519.PublicKeySize {
			return nil, fmt.Errorf("license: public key %q has wrong size (%d, want %d)", kid, len(key), ed25519.PublicKeySize)
		}
		out[kid] = ed25519.PublicKey(key)
	}
	return out, nil
}
