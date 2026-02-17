package container

import (
	"encoding/binary"
	"io"
)

// stdCopy demultiplexes the Docker container log stream.
// Docker prefixes each output frame with an 8-byte header:
// [stream_type(1)][padding(3)][size(4 big-endian)]
// stream_type: 1 = stdout, 2 = stderr.
func stdCopy(stdout, stderr io.Writer, src io.Reader) (int64, error) {
	var total int64
	header := make([]byte, 8)

	for {
		_, err := io.ReadFull(src, header)
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}

		streamType := header[0]
		frameSize := binary.BigEndian.Uint32(header[4:8])

		var dst io.Writer
		switch streamType {
		case 1:
			dst = stdout
		case 2:
			dst = stderr
		default:
			dst = stdout
		}

		n, err := io.CopyN(dst, src, int64(frameSize))
		total += n
		if err != nil {
			return total, err
		}
	}
}
