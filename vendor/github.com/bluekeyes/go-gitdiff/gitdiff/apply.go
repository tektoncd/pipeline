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
//	    if errors.Is(err, &Conflict{}) {
//		       // handle conflict
//	    }
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
	errApplierClosed   = errors.New("gitdiff: applier is closed")
)

// Apply applies the changes in f to src, writing the result to dst. It can
// apply both text and binary changes.
//
// If an error occurs while applying, Apply returns an *ApplyError that
// annotates the error with additional information. If the error is because of
// a conflict with the source, the wrapped error will be a *Conflict.
func Apply(dst io.Writer, src io.ReaderAt, f *File) error {
	if f.IsBinary {
		if len(f.TextFragments) > 0 {
			return applyError(errors.New("binary file contains text fragments"))
		}
		if f.BinaryFragment == nil {
			return applyError(errors.New("binary file does not contain a binary fragment"))
		}
	} else {
		if f.BinaryFragment != nil {
			return applyError(errors.New("text file contains a binary fragment"))
		}
	}

	switch {
	case f.BinaryFragment != nil:
		applier := NewBinaryApplier(dst, src)
		if err := applier.ApplyFragment(f.BinaryFragment); err != nil {
			return err
		}
		return applier.Close()

	case len(f.TextFragments) > 0:
		frags := make([]*TextFragment, len(f.TextFragments))
		copy(frags, f.TextFragments)

		sort.Slice(frags, func(i, j int) bool {
			return frags[i].OldPosition < frags[j].OldPosition
		})

		// TODO(bkeyes): consider merging overlapping fragments
		// right now, the application fails if fragments overlap, but it should be
		// possible to precompute the result of applying them in order

		applier := NewTextApplier(dst, src)
		for i, frag := range frags {
			if err := applier.ApplyFragment(frag); err != nil {
				return applyError(err, fragNum(i))
			}
		}
		return applier.Close()

	default:
		// nothing to apply, just copy all the data
		_, err := copyFrom(dst, src, 0)
		return err
	}
}
