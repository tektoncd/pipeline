package gitdiff

import (
	"errors"
	"io"
)

const (
	byteBufferSize  = 32 * 1024 // from io.Copy
	lineBufferSize  = 32
	indexBufferSize = 1024
)

// LineReaderAt is the interface that wraps the ReadLinesAt method.
//
// ReadLinesAt reads len(lines) into lines starting at line offset. It returns
// the number of lines read (0 <= n <= len(lines)) and any error encountered.
// Line numbers are zero-indexed.
//
// If n < len(lines), ReadLinesAt returns a non-nil error explaining why more
// lines were not returned.
//
// Lines read by ReadLinesAt include the newline character. The last line does
// not have a final newline character if the input ends without one.
type LineReaderAt interface {
	ReadLinesAt(lines [][]byte, offset int64) (n int, err error)
}

type lineReaderAt struct {
	r     io.ReaderAt
	index []int64
	eof   bool
}

func (r *lineReaderAt) ReadLinesAt(lines [][]byte, offset int64) (n int, err error) {
	if offset < 0 {
		return 0, errors.New("ReadLinesAt: negative offset")
	}
	if len(lines) == 0 {
		return 0, nil
	}

	count := len(lines)
	startLine := offset
	endLine := startLine + int64(count)

	if endLine > int64(len(r.index)) && !r.eof {
		if err := r.indexTo(endLine); err != nil {
			return 0, err
		}
	}
	if startLine >= int64(len(r.index)) {
		return 0, io.EOF
	}

	buf, byteOffset, err := r.readBytes(startLine, int64(count))
	if err != nil {
		return 0, err
	}

	for n = 0; n < count && startLine+int64(n) < int64(len(r.index)); n++ {
		lineno := startLine + int64(n)
		start, end := int64(0), r.index[lineno]-byteOffset
		if lineno > 0 {
			start = r.index[lineno-1] - byteOffset
		}
		lines[n] = buf[start:end]
	}

	if n < count {
		return n, io.EOF
	}
	return n, nil
}

// indexTo reads data and computes the line index until there is information
// for line or a read returns io.EOF. It returns an error if and only if there
// is an error reading data.
func (r *lineReaderAt) indexTo(line int64) error {
	var buf [indexBufferSize]byte

	offset := r.lastOffset()
	for int64(len(r.index)) < line {
		n, err := r.r.ReadAt(buf[:], offset)
		if err != nil && err != io.EOF {
			return err
		}
		for _, b := range buf[:n] {
			offset++
			if b == '\n' {
				r.index = append(r.index, offset)
			}
		}
		if err == io.EOF {
			if offset > r.lastOffset() {
				r.index = append(r.index, offset)
			}
			r.eof = true
			break
		}
	}
	return nil
}

func (r *lineReaderAt) lastOffset() int64 {
	if n := len(r.index); n > 0 {
		return r.index[n-1]
	}
	return 0
}

// readBytes reads the bytes of the n lines starting at line and returns the
// bytes and the offset of the first byte in the underlying source.
func (r *lineReaderAt) readBytes(line, n int64) (b []byte, offset int64, err error) {
	indexLen := int64(len(r.index))

	var size int64
	if line > indexLen {
		offset = r.index[indexLen-1]
	} else if line > 0 {
		offset = r.index[line-1]
	}
	if n > 0 {
		if line+n > indexLen {
			size = r.index[indexLen-1] - offset
		} else {
			size = r.index[line+n-1] - offset
		}
	}

	b = make([]byte, size)
	if _, err := r.r.ReadAt(b, offset); err != nil {
		if err == io.EOF {
			err = errors.New("ReadLinesAt: corrupt line index or changed source data")
		}
		return nil, 0, err
	}
	return b, offset, nil
}

func isLen(r io.ReaderAt, n int64) (bool, error) {
	off := n - 1
	if off < 0 {
		off = 0
	}

	var b [2]byte
	nr, err := r.ReadAt(b[:], off)
	if err == io.EOF {
		return (n == 0 && nr == 0) || (n > 0 && nr == 1), nil
	}
	return false, err
}

// copyFrom writes bytes starting from offset off in src to dst stopping at the
// end of src or at the first error. copyFrom returns the number of bytes
// written and any error.
func copyFrom(dst io.Writer, src io.ReaderAt, off int64) (written int64, err error) {
	buf := make([]byte, byteBufferSize)
	for {
		nr, rerr := src.ReadAt(buf, off)
		if nr > 0 {
			nw, werr := dst.Write(buf[0:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if werr != nil {
				err = werr
				break
			}
			if nr != nw {
				err = io.ErrShortWrite
				break
			}
			off += int64(nr)
		}
		if rerr != nil {
			if rerr != io.EOF {
				err = rerr
			}
			break
		}
	}
	return written, err
}

// copyLinesFrom writes lines starting from line off in src to dst stopping at
// the end of src or at the first error. copyLinesFrom returns the number of
// lines written and any error.
func copyLinesFrom(dst io.Writer, src LineReaderAt, off int64) (written int64, err error) {
	buf := make([][]byte, lineBufferSize)
ReadLoop:
	for {
		nr, rerr := src.ReadLinesAt(buf, off)
		if nr > 0 {
			for _, line := range buf[0:nr] {
				nw, werr := dst.Write(line)
				if nw > 0 {
					written++
				}
				if werr != nil {
					err = werr
					break ReadLoop
				}
				if len(line) != nw {
					err = io.ErrShortWrite
					break ReadLoop
				}
			}
			off += int64(nr)
		}
		if rerr != nil {
			if rerr != io.EOF {
				err = rerr
			}
			break
		}
	}
	return written, err
}
