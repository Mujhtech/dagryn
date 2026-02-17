package remote

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// Digest represents a content-addressable hash.
type Digest struct {
	Hash string // hex-encoded SHA256
	Size int64
}

// Key returns the CAS storage key for this digest: cas/{hash[:2]}/{hash}.
func (d Digest) Key() string {
	if len(d.Hash) < 2 {
		return fmt.Sprintf("cas/%s", d.Hash)
	}
	return fmt.Sprintf("cas/%s/%s", d.Hash[:2], d.Hash)
}

// DigestBytes computes a Digest from a byte slice.
func DigestBytes(data []byte) Digest {
	h := sha256.Sum256(data)
	return Digest{
		Hash: hex.EncodeToString(h[:]),
		Size: int64(len(data)),
	}
}

// DigestReader computes a Digest by reading from r.
// It reads the entire stream and returns the digest.
func DigestReader(r io.Reader) (Digest, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	if err != nil {
		return Digest{}, fmt.Errorf("digest: read: %w", err)
	}
	return Digest{
		Hash: hex.EncodeToString(h.Sum(nil)),
		Size: n,
	}, nil
}
