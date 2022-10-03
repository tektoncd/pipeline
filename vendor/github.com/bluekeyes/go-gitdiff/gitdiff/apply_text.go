package gitdiff

import (
	"io"
)

// TextApplier applies changes described in text fragments to source data. If
// changes are described in multiple fragments, those fragments must be applied
// in order. The applier must be closed after use.
//
// By default, TextApplier operates in "strict" mode, where fragment content
// and positions must exactly match those of the source.
type TextApplier struct {
	dst      io.Writer
	src      io.ReaderAt
	lineSrc  LineReaderAt
	nextLine int64

	closed bool
	dirty  bool
}

// NewTextApplier creates a TextApplier that reads data from src and writes
// modified data to dst. If src implements LineReaderAt, it is used directly.
func NewTextApplier(dst io.Writer, src io.ReaderAt) *TextApplier {
	a := TextApplier{
		dst: dst,
		src: src,
	}

	if lineSrc, ok := src.(LineReaderAt); ok {
		a.lineSrc = lineSrc
	} else {
		a.lineSrc = &lineReaderAt{r: src}
	}

	return &a
}

// ApplyFragment applies the changes in the fragment f, writing unwritten data
// before the start of the fragment and any changes from the fragment. If
// multiple text fragments apply to the same content, ApplyFragment must be
// called in order of increasing start position. As a result, each fragment can
// be applied at most once.
//
// If an error occurs while applying, ApplyFragment returns an *ApplyError that
// annotates the error with additional information. If the error is because of
// a conflict between the fragment and the source, the wrapped error will be a
// *Conflict.
func (a *TextApplier) ApplyFragment(f *TextFragment) error {
	if a.closed {
		return applyError(errApplierClosed)
	}

	// mark an apply as in progress, even if it fails before making changes
	a.dirty = true

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
		if _, err := a.dst.Write(line); err != nil {
			a.nextLine = start + int64(i)
			return applyError(err, lineNum(a.nextLine))
		}
	}
	preimage = preimage[fragStart-start:]

	// apply the changes in the fragment
	used := int64(0)
	for i, line := range f.Lines {
		if err := applyTextLine(a.dst, line, preimage, used); err != nil {
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

// Close writes any data following the last applied fragment and prevents
// future calls to ApplyFragment.
func (a *TextApplier) Close() (err error) {
	if a.closed {
		return nil
	}

	a.closed = true
	if !a.dirty {
		_, err = copyFrom(a.dst, a.src, 0)
	} else {
		_, err = copyLinesFrom(a.dst, a.lineSrc, a.nextLine)
	}
	return err
}
