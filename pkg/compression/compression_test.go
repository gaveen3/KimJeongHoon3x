package compression

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"

	"github.com/pkg/errors"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDetectCompression(t *testing.T) {
	cases := []string{
		"fixtures/Hello.uncompressed",
		"fixtures/Hello.gz",
		"fixtures/Hello.bz2",
		"fixtures/Hello.xz",
	}

	// The original stream is preserved.
	for _, c := range cases {
		originalContents, err := ioutil.ReadFile(c)
		require.NoError(t, err, c)

		stream, err := os.Open(c)
		require.NoError(t, err, c)
		defer stream.Close()

		_, updatedStream, err := DetectCompression(stream)
		require.NoError(t, err, c)

		updatedContents, err := ioutil.ReadAll(updatedStream)
		require.NoError(t, err, c)
		assert.Equal(t, originalContents, updatedContents, c)
	}

	// The correct decompressor is chosen, and the result is as expected.
	for _, c := range cases {
		stream, err := os.Open(c)
		require.NoError(t, err, c)
		defer stream.Close()

		decompressor, updatedStream, err := DetectCompression(stream)
		require.NoError(t, err, c)

		var uncompressedStream io.Reader
		if decompressor == nil {
			uncompressedStream = updatedStream
		} else {
			s, err := decompressor(updatedStream)
			require.NoError(t, err)
			defer s.Close()
			uncompressedStream = s
		}

		uncompressedContents, err := ioutil.ReadAll(uncompressedStream)
		require.NoError(t, err, c)
		assert.Equal(t, []byte("Hello"), uncompressedContents, c)
	}

	// Empty input is handled reasonably.
	decompressor, updatedStream, err := DetectCompression(bytes.NewReader([]byte{}))
	require.NoError(t, err)
	assert.Nil(t, decompressor)
	updatedContents, err := ioutil.ReadAll(updatedStream)
	require.NoError(t, err)
	assert.Equal(t, []byte{}, updatedContents)

	// Error reading input
	reader, writer := io.Pipe()
	defer reader.Close()
	writer.CloseWithError(errors.New("Expected error reading input in DetectCompression"))
	_, _, err = DetectCompression(reader)
	assert.Error(t, err)
}
