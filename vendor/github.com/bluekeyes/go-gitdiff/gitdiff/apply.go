package gitdiff

import (
	"errors"
	"fmt"
	"io"
	"sort"
)

// Conflict indicates an apply failed due to a conflict between the patch and
// the source content.
//
// Users can test if an error was caused by a conflict by using errors.Is with
// an empty Conflict:
//
//     if errors.Is(err, &Conflict{}) {
//	       // handle conflict
//     }
//
type Conflict struct {
	msg string
}

func (c *Conflict) Error() string {
	return "conflict: " + c.msg
}

// Is implements error matching for Conflict. Passing an empty instance of
// Conflict always returns true.
func (c *Conflict) Is(other error) bool {
	if other, ok := other.(*Conflict); ok {
		return other.msg == "" || other.msg == c.msg
	}
	return false
}

// ApplyError wraps an error that occurs during patch application with
// additional location information, if it is available.
type ApplyError struct {
	// Line is the one-indexed line number in the source data
	Line int64
	// Fragment is the one-indexed fragment number in the file
	Fragment int
	// FragmentLine is the one-indexed line number in the fragment
	FragmentLine int

	err error
}

// Unwrap returns the wrapped error.
func (e *ApplyError) Unwrap() error {
	return e.err
}

func (e *ApplyError) Error() string {
	return fmt.Sprintf("%v", e.err)
}

type lineNum int
type fragNum int
type fragLineNum int

// applyError creates a new *ApplyError wrapping err or augments the information
// in err with args if it is already an *ApplyError. Returns nil if err is nil.
func applyError(err error, args ...interface{}) error {
	if err == nil {
		return nil
	}

	e, ok := err.(*ApplyError)
	if !ok {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		e = &ApplyError{err: err}
	}
	for _, arg := range args {
		switch v := arg.(type) {
		case lineNum:
			e.Line = int64(v) + 1
		case fragNum:
			e.Fragment = int(v) + 1
		case fragLineNum:
			e.FragmentLine = int(v) + 1
		}
	}
	return e
}

var (
	errApplyInProgress = errors.New("gitdiff: incompatible apply in progress")
)

const (
	applyInitial = iota
	applyText
	applyBinary
	applyFile
)

// Apply is a convenience function that creates an Applier for src with default
// settings and applies the changes in f, writing the result to dst.
func Apply(dst io.Writer, src io.ReaderAt, f *File) error {
	return NewApplier(src).ApplyFile(dst, f)
}

// Applier applies changes described in fragments to source data. If changes
// are described in multiple fragments, those fragments must be applied in
// order, usually by calling ApplyFile.
//
// By default, Applier operates in "strict" mode, where fragment content and
// positions must exactly match those of the source.
//
// If an error occurs while applying, methods on Applier return instances of
// *ApplyError that annotate the wrapped error with additional information
// when available. If the error is because of a conflict between a fragment and
// the source, the wrapped error will be a *Conflict.
//
// While an Applier can apply both text and binary fragments, only one fragment
// type can be used without resetting the Applier. The first fragment applied
// sets the type for the Applier. Mixing fragment types or mixing
// fragment-level and file-level applies results in an error.
type Applier struct {
	src       io.ReaderAt
	lineSrc   LineReaderAt
	nextLine  int64
	applyType int
}

// NewApplier creates an Applier that reads data from src. If src is a
// LineReaderAt, it is used directly to apply text fragments.
func NewApplier(src io.ReaderAt) *Applier {
	a := new(Applier)
	a.Reset(src)
	return a
}

// Reset resets the input and internal state of the Applier. If src is nil, the
// existing source is reused.
func (a *Applier) Reset(src io.ReaderAt) {
	if src != nil {
		a.src = src
		if lineSrc, ok := src.(LineReaderAt); ok {
			a.lineSrc = lineSrc
		} else {
			a.lineSrc = &lineReaderAt{r: src}
		}
	}
	a.nextLine = 0
	a.applyType = applyInitial
}

// ApplyFile applies the changes in all of the fragments of f and writes the
// result to dst.
func (a *Applier) ApplyFile(dst io.Writer, f *File) error {
	if a.applyType != applyInitial {
		return applyError(errApplyInProgress)
	}
	defer func() { a.applyType = applyFile }()

	if f.IsBinary && len(f.TextFragments) > 0 {
		return applyError(errors.New("binary file contains text fragments"))
	}
	if !f.IsBinary && f.BinaryFragment != nil {
		return applyError(errors.New("text file contains binary fragment"))
	}

	switch {
	case f.BinaryFragment != nil:
		return a.ApplyBinaryFragment(dst, f.BinaryFragment)

	case len(f.TextFragments) > 0:
		frags := make([]*TextFragment, len(f.TextFragments))
		copy(frags, f.TextFragments)

		sort.Slice(frags, func(i, j int) bool {
			return frags[i].OldPosition < frags[j].OldPosition
		})

		// TODO(bkeyes): consider merging overlapping fragments
		// right now, the application fails if fragments overlap, but it should be
		// possible to precompute the result of applying them in order

		for i, frag := range frags {
			if err := a.ApplyTextFragment(dst, frag); err != nil {
				return applyError(err, fragNum(i))
			}
		}
	}

	return applyError(a.Flush(dst))
}

// ApplyTextFragment applies the changes in the fragment f and writes unwritten
// data before the start of the fragment and the result to dst. If multiple
// text fragments apply to the same source, ApplyTextFragment must be called in
// order of increasing start position. As a result, each fragment can be
// applied at most once before a call to Reset.
func (a *Applier) ApplyTextFragment(dst io.Writer, f *TextFragment) error {
	if a.applyType != applyInitial && a.applyType != applyText {
		return applyError(errApplyInProgress)
	}
	defer func() { a.applyType = applyText }()

	// application code assumes fragment fields are consistent
	if err := f.Validate(); err != nil {
		return applyError(err)
	}

	// lines are 0-indexed, positions are 1-indexed (but new files have position = 0)
	fragStart := f.OldPosition - 1
	if fragStart < 0 {
		fragStart = 0
	}
	fragEnd := fragStart + f.OldLines

	start := a.nextLine
	if fragStart < start {
		return applyError(&Conflict{"fragment overlaps with an applied fragment"})
	}

	if f.OldPosition == 0 {
		ok, err := isLen(a.src, 0)
		if err != nil {
			return applyError(err)
		}
		if !ok {
			return applyError(&Conflict{"cannot create new file from non-empty src"})
		}
	}

	preimage := make([][]byte, fragEnd-start)
	n, err := a.lineSrc.ReadLinesAt(preimage, start)
	if err != nil {
		return applyError(err, lineNum(start+int64(n)))
	}

	// copy leading data before the fragment starts
	for i, line := range preimage[:fragStart-start] {
		if _, err := dst.Write(line); err != nil {
			a.nextLine = start + int64(i)
			return applyError(err, lineNum(a.nextLine))
		}
	}
	preimage = preimage[fragStart-start:]

	// apply the changes in the fragment
	used := int64(0)
	for i, line := range f.Lines {
		if err := applyTextLine(dst, line, preimage, used); err != nil {
			a.nextLine = fragStart + used
			return applyError(err, lineNum(a.nextLine), fragLineNum(i))
		}
		if line.Old() {
			used++
		}
	}
	a.nextLine = fragStart + used

	// new position of +0,0 mean a full delete, so check for leftovers
	if f.NewPosition == 0 && f.NewLines == 0 {
		var b [1][]byte
		n, err := a.lineSrc.ReadLinesAt(b[:], a.nextLine)
		if err != nil && err != io.EOF {
			return applyError(err, lineNum(a.nextLine))
		}
		if n > 0 {
			return applyError(&Conflict{"src still has content after full delete"}, lineNum(a.nextLine))
		}
	}

	return nil
}

func applyTextLine(dst io.Writer, line Line, preimage [][]byte, i int64) (err error) {
	if line.Old() && string(preimage[i]) != line.Line {
		return &Conflict{"fragment line does not match src line"}
	}
	if line.New() {
		_, err = io.WriteString(dst, line.Line)
	}
	return err
}

// Flush writes any data following the last applied fragment to dst.
func (a *Applier) Flush(dst io.Writer) (err error) {
	switch a.applyType {
	case applyInitial:
		_, err = copyFrom(dst, a.src, 0)
	case applyText:
		_, err = copyLinesFrom(dst, a.lineSrc, a.nextLine)
	case applyBinary:
		// nothing to flush, binary apply "consumes" full source
	}
	return err
}

// ApplyBinaryFragment applies the changes in the fragment f and writes the
// result to dst. At most one binary fragment can be applied before a call to
// Reset.
func (a *Applier) ApplyBinaryFragment(dst io.Writer, f *BinaryFragment) error {
	if a.applyType != applyInitial {
		return applyError(errApplyInProgress)
	}
	defer func() { a.applyType = applyBinary }()

	if f == nil {
		return applyError(errors.New("nil fragment"))
	}

	switch f.Method {
	case BinaryPatchLiteral:
		if _, err := dst.Write(f.Data); err != nil {
			return applyError(err)
		}
	case BinaryPatchDelta:
		if err := applyBinaryDeltaFragment(dst, a.src, f.Data); err != nil {
			return applyError(err)
		}
	default:
		return applyError(fmt.Errorf("unsupported binary patch method: %v", f.Method))
	}
	return nil
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
//    [[1xxxxxxx]...] [0xxxxxxx]
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
//     [0xxxxxx][[data1]...]
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
//     [1xxxxxxx][offset1][offset2][offset3][offset4][size1][size2][size3]
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
