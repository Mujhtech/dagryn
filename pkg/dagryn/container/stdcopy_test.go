package container

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
)

func buildDockerFrame(streamType byte, data []byte) []byte {
	header := make([]byte, 8)
	header[0] = streamType
	binary.BigEndian.PutUint32(header[4:], uint32(len(data)))
	return append(header, data...)
}

func TestStdCopy(t *testing.T) {
	t.Run("stdout only", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		frame := buildDockerFrame(1, []byte("hello stdout"))

		n, err := stdCopy(&stdout, &stderr, bytes.NewReader(frame))
		assert.NoError(t, err)
		assert.Equal(t, int64(12), n)
		assert.Equal(t, "hello stdout", stdout.String())
		assert.Empty(t, stderr.String())
	})

	t.Run("stderr only", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		frame := buildDockerFrame(2, []byte("hello stderr"))

		n, err := stdCopy(&stdout, &stderr, bytes.NewReader(frame))
		assert.NoError(t, err)
		assert.Equal(t, int64(12), n)
		assert.Empty(t, stdout.String())
		assert.Equal(t, "hello stderr", stderr.String())
	})

	t.Run("interleaved stdout and stderr", func(t *testing.T) {
		var stdout, stderr bytes.Buffer

		var input bytes.Buffer
		input.Write(buildDockerFrame(1, []byte("out1")))
		input.Write(buildDockerFrame(2, []byte("err1")))
		input.Write(buildDockerFrame(1, []byte("out2")))

		n, err := stdCopy(&stdout, &stderr, &input)
		assert.NoError(t, err)
		assert.Equal(t, int64(12), n) // 4 + 4 + 4
		assert.Equal(t, "out1out2", stdout.String())
		assert.Equal(t, "err1", stderr.String())
	})

	t.Run("empty input", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		n, err := stdCopy(&stdout, &stderr, bytes.NewReader(nil))
		assert.NoError(t, err)
		assert.Equal(t, int64(0), n)
	})
}
