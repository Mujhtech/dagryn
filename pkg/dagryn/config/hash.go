package config

import (
	"crypto/sha256"
	"encoding/hex"
)

// ComputeConfigHash returns a hex-encoded SHA-256 hash of the raw config bytes.
// This is used for diff detection when syncing workflows — if the hash hasn't
// changed, the workflow version is not bumped.
func ComputeConfigHash(rawConfig []byte) string {
	h := sha256.Sum256(rawConfig)
	return hex.EncodeToString(h[:])
}
