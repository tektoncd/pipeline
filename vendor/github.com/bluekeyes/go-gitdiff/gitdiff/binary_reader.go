package gitdiff

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
)

// binaryReader reads the compressed raw data in a binary fragment,
// decompressing it and checking it against the expected size. Clients should
// read until it returns an error or io.EOF.
type binaryReader struct {
	r    *io.LimitedReader
	c    io.Closer
	size int64
	raw  []byte
}

func newBinaryReader(f *BinaryFragment) io.Reader {
	return &binaryReader{
		raw:  f.RawData,
		size: f.Size,
	}
}

func (r *binaryReader) Read(p []byte) (int, error) {
	// Defer initialization of the zlib reader so that we report any header
	// errors from Read() instead of when creating the binaryReader. This
	// allows clients to use io.ReadAll() directly with BinaryFragment.Data().
	if r.r == nil {
		zr, err := zlib.NewReader(bytes.NewReader(r.raw))
		if err != nil {
			return 0, err
		}
		r.r = &io.LimitedReader{R: zr, N: r.size + 1}
		r.c = zr
	}

	n, err := r.r.Read(p)
	if err == io.EOF {
		// If we reached the "end", first check that we read the correct amount
		// of data. On an exact read, the limit reader has one byte remaining.
		switch {
		case r.r.N > 1:
			return n, fmt.Errorf("incorrect inflated size: expected %d bytes, received %d", r.size, r.size-(r.r.N-1))
		case r.r.N < 1:
			return n, fmt.Errorf("incorrect inflated size: expected %d bytes, received more", r.size)
		}
		// Then check that the zlib checksum is valid by closing the reader
		if err := r.c.Close(); err != nil {
			return n, err
		}
	}
	return n, err
}
