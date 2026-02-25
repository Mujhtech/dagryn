package cloud

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifyReader_ValidData(t *testing.T) {
	data := "hello world"
	hash := sha256Hex([]byte(data))

	vr := newVerifyReader(strings.NewReader(data), hash, int64(len(data)))
	got, err := io.ReadAll(vr)
	require.NoError(t, err)
	assert.Equal(t, data, string(got))
	assert.NoError(t, vr.Verify())
}

func TestVerifyReader_HashMismatch(t *testing.T) {
	data := "hello world"
	wrongHash := sha256Hex([]byte("wrong data"))

	vr := newVerifyReader(strings.NewReader(data), wrongHash, int64(len(data)))
	_, err := io.ReadAll(vr)
	require.NoError(t, err)

	err = vr.Verify()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "checksum mismatch")
}

func TestVerifyReader_SizeMismatch(t *testing.T) {
	data := "hello world"
	hash := sha256Hex([]byte(data))

	vr := newVerifyReader(strings.NewReader(data), hash, int64(len(data)+100))
	_, err := io.ReadAll(vr)
	require.NoError(t, err)

	err = vr.Verify()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

func TestVerifyReader_EmptyHash_NoOp(t *testing.T) {
	data := "anything"
	vr := newVerifyReader(strings.NewReader(data), "", 0)
	_, err := io.ReadAll(vr)
	require.NoError(t, err)
	assert.NoError(t, vr.Verify())
}

func TestVerifyReader_ZeroExpectSize_SkipsSizeCheck(t *testing.T) {
	data := "hello world"
	hash := sha256Hex([]byte(data))

	vr := newVerifyReader(strings.NewReader(data), hash, 0)
	_, err := io.ReadAll(vr)
	require.NoError(t, err)
	assert.NoError(t, vr.Verify())
}

func TestVerifyReader_Truncated(t *testing.T) {
	full := "hello world, this is complete data"
	hash := sha256Hex([]byte(full))
	truncated := full[:10]

	vr := newVerifyReader(strings.NewReader(truncated), hash, int64(len(full)))
	_, err := io.ReadAll(vr)
	require.NoError(t, err)

	err = vr.Verify()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "size mismatch")
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
