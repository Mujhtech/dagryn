package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
)

// verifyReader wraps an io.Reader, computing a SHA256 hash of all bytes that
// flow through it. After the underlying reader is fully consumed, call Verify()
// to check the accumulated digest and byte count against expected values.
type verifyReader struct {
	r          io.Reader
	hasher     hash.Hash
	expected   string
	expectSize int64
	n          int64
}

func newVerifyReader(r io.Reader, expectedHash string, expectedSize int64) *verifyReader {
	return &verifyReader{
		r:          r,
		hasher:     sha256.New(),
		expected:   expectedHash,
		expectSize: expectedSize,
	}
}

func (v *verifyReader) Read(p []byte) (int, error) {
	n, err := v.r.Read(p)
	if n > 0 {
		v.hasher.Write(p[:n])
		v.n += int64(n)
	}
	return n, err
}

// Verify checks that the accumulated digest and byte count match the expected values.
// It is a no-op (returns nil) when no expected hash was provided, which keeps
// backward compatibility with servers that don't send the X-Checksum-SHA256 header.
func (v *verifyReader) Verify() error {
	if v.expected == "" {
		return nil
	}
	if v.expectSize > 0 && v.n != v.expectSize {
		return fmt.Errorf("size mismatch: expected %d bytes, got %d", v.expectSize, v.n)
	}
	actual := hex.EncodeToString(v.hasher.Sum(nil))
	if actual != v.expected {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", v.expected, actual)
	}
	return nil
}
