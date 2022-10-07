package gitdiff

import (
	"errors"
	"fmt"
	"io"
)

// BinaryApplier applies binary changes described in a fragment to source data.
// The applier must be closed after use.
type BinaryApplier struct {
	dst io.Writer
	src io.ReaderAt

	closed bool
	dirty  bool
}

// NewBinaryApplier creates an BinaryApplier that reads data from src and
// writes modified data to dst.
func NewBinaryApplier(dst io.Writer, src io.ReaderAt) *BinaryApplier {
	a := BinaryApplier{
		dst: dst,
		src: src,
	}
	return &a
}

// ApplyFragment applies the changes in the fragment f and writes the result to
// dst. ApplyFragment can be called at most once.
//
// If an error occurs while applying, ApplyFragment returns an *ApplyError that
// annotates the error with additional information. If the error is because of
// a conflict between a fragment and the source, the wrapped error will be a
// *Conflict.
func (a *BinaryApplier) ApplyFragment(f *BinaryFragment) error {
	if f == nil {
		return applyError(errors.New("nil fragment"))
	}
	if a.closed {
		return applyError(errApplierClosed)
	}
	if a.dirty {
		return applyError(errApplyInProgress)
	}

	// mark an apply as in progress, even if it fails before making changes
	a.dirty = true

	switch f.Method {
	case BinaryPatchLiteral:
		if _, err := a.dst.Write(f.Data); err != nil {
			return applyError(err)
		}
	case BinaryPatchDelta:
		if err := applyBinaryDeltaFragment(a.dst, a.src, f.Data); err != nil {
			return applyError(err)
		}
	default:
		return applyError(fmt.Errorf("unsupported binary patch method: %v", f.Method))
	}
	return nil
}

// Close writes any data following the last applied fragment and prevents
// future calls to ApplyFragment.
func (a *BinaryApplier) Close() (err error) {
	if a.closed {
		return nil
	}

	a.closed = true
	if !a.dirty {
		_, err = copyFrom(a.dst, a.src, 0)
	} else {
		// do nothing, applying a binary fragment copies all data
	}
	return err
}

func applyBinaryDeltaFragment(dst io.Writer, src io.ReaderAt, frag []byte) error {
	srcSize, delta := readBinaryDeltaSize(frag)
	if err := checkBinarySrcSize(src, srcSize); err != nil {
		return err
	}

	dstSize, delta := readBinaryDeltaSize(delta)

	for len(delta) > 0 {
		op := delta[0]
		if op == 0 {
			return errors.New("invalid delta opcode 0")
		}

		var n int64
		var err error
		switch op & 0x80 {
		case 0x80:
			n, delta, err = applyBinaryDeltaCopy(dst, op, delta[1:], src)
		case 0x00:
			n, delta, err = applyBinaryDeltaAdd(dst, op, delta[1:])
		}
		if err != nil {
			return err
		}
		dstSize -= n
	}

	if dstSize != 0 {
		return errors.New("corrupt binary delta: insufficient or extra data")
	}
	return nil
}

// readBinaryDeltaSize reads a variable length size from a delta-encoded binary
// fragment, returing the size and the unused data. Data is encoded as:
//
//	[[1xxxxxxx]...] [0xxxxxxx]
//
// in little-endian order, with 7 bits of the value per byte.
func readBinaryDeltaSize(d []byte) (size int64, rest []byte) {
	shift := uint(0)
	for i, b := range d {
		size |= int64(b&0x7F) << shift
		shift += 7
		if b <= 0x7F {
			return size, d[i+1:]
		}
	}
	return size, nil
}

// applyBinaryDeltaAdd applies an add opcode in a delta-encoded binary
// fragment, returning the amount of data written and the usused part of the
// fragment. An add operation takes the form:
//
//	[0xxxxxx][[data1]...]
//
// where the lower seven bits of the opcode is the number of data bytes
// following the opcode. See also pack-format.txt in the Git source.
func applyBinaryDeltaAdd(w io.Writer, op byte, delta []byte) (n int64, rest []byte, err error) {
	size := int(op)
	if len(delta) < size {
		return 0, delta, errors.New("corrupt binary delta: incomplete add")
	}
	_, err = w.Write(delta[:size])
	return int64(size), delta[size:], err
}

// applyBinaryDeltaCopy applies a copy opcode in a delta-encoded binary
// fragment, returing the amount of data written and the unused part of the
// fragment. A copy operation takes the form:
//
//	[1xxxxxxx][offset1][offset2][offset3][offset4][size1][size2][size3]
//
// where the lower seven bits of the opcode determine which non-zero offset and
// size bytes are present in little-endian order: if bit 0 is set, offset1 is
// present, etc. If no offset or size bytes are present, offset is 0 and size
// is 0x10000. See also pack-format.txt in the Git source.
func applyBinaryDeltaCopy(w io.Writer, op byte, delta []byte, src io.ReaderAt) (n int64, rest []byte, err error) {
	const defaultSize = 0x10000

	unpack := func(start, bits uint) (v int64) {
		for i := uint(0); i < bits; i++ {
			mask := byte(1 << (i + start))
			if op&mask > 0 {
				if len(delta) == 0 {
					err = errors.New("corrupt binary delta: incomplete copy")
					return
				}
				v |= int64(delta[0]) << (8 * i)
				delta = delta[1:]
			}
		}
		return
	}

	offset := unpack(0, 4)
	size := unpack(4, 3)
	if err != nil {
		return 0, delta, err
	}
	if size == 0 {
		size = defaultSize
	}

	// TODO(bkeyes): consider pooling these buffers
	b := make([]byte, size)
	if _, err := src.ReadAt(b, offset); err != nil {
		return 0, delta, err
	}

	_, err = w.Write(b)
	return size, delta, err
}

func checkBinarySrcSize(r io.ReaderAt, size int64) error {
	ok, err := isLen(r, size)
	if err != nil {
		return err
	}
	if !ok {
		return &Conflict{"fragment src size does not match actual src size"}
	}
	return nil
}
