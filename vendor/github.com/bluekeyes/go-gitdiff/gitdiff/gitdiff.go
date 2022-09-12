package gitdiff

import (
	"errors"
	"fmt"
	"os"
)

// File describes changes to a single file. It can be either a text file or a
// binary file.
type File struct {
	OldName string
	NewName string

	IsNew    bool
	IsDelete bool
	IsCopy   bool
	IsRename bool

	OldMode os.FileMode
	NewMode os.FileMode

	OldOIDPrefix string
	NewOIDPrefix string
	Score        int

	// TextFragments contains the fragments describing changes to a text file. It
	// may be empty if the file is empty or if only the mode changes.
	TextFragments []*TextFragment

	// IsBinary is true if the file is a binary file. If the patch includes
	// binary data, BinaryFragment will be non-nil and describe the changes to
	// the data. If the patch is reversible, ReverseBinaryFragment will also be
	// non-nil and describe the changes needed to restore the original file
	// after applying the changes in BinaryFragment.
	IsBinary              bool
	BinaryFragment        *BinaryFragment
	ReverseBinaryFragment *BinaryFragment
}

// TextFragment describes changed lines starting at a specific line in a text file.
type TextFragment struct {
	Comment string

	OldPosition int64
	OldLines    int64

	NewPosition int64
	NewLines    int64

	LinesAdded   int64
	LinesDeleted int64

	LeadingContext  int64
	TrailingContext int64

	Lines []Line
}

// Header returns the canonical header of this fragment.
func (f *TextFragment) Header() string {
	return fmt.Sprintf("@@ -%d,%d +%d,%d @@ %s", f.OldPosition, f.OldLines, f.NewPosition, f.NewLines, f.Comment)
}

// Validate checks that the fragment is self-consistent and appliable. Validate
// returns an error if and only if the fragment is invalid.
func (f *TextFragment) Validate() error {
	if f == nil {
		return errors.New("nil fragment")
	}

	var (
		oldLines, newLines                     int64
		leadingContext, trailingContext        int64
		contextLines, addedLines, deletedLines int64
	)

	// count the types of lines in the fragment content
	for i, line := range f.Lines {
		switch line.Op {
		case OpContext:
			oldLines++
			newLines++
			contextLines++
			if addedLines == 0 && deletedLines == 0 {
				leadingContext++
			} else {
				trailingContext++
			}
		case OpAdd:
			newLines++
			addedLines++
			trailingContext = 0
		case OpDelete:
			oldLines++
			deletedLines++
			trailingContext = 0
		default:
			return fmt.Errorf("unknown operator %q on line %d", line.Op, i+1)
		}
	}

	// check the actual counts against the reported counts
	if oldLines != f.OldLines {
		return lineCountErr("old", oldLines, f.OldLines)
	}
	if newLines != f.NewLines {
		return lineCountErr("new", newLines, f.NewLines)
	}
	if leadingContext != f.LeadingContext {
		return lineCountErr("leading context", leadingContext, f.LeadingContext)
	}
	if trailingContext != f.TrailingContext {
		return lineCountErr("trailing context", trailingContext, f.TrailingContext)
	}
	if addedLines != f.LinesAdded {
		return lineCountErr("added", addedLines, f.LinesAdded)
	}
	if deletedLines != f.LinesDeleted {
		return lineCountErr("deleted", deletedLines, f.LinesDeleted)
	}

	// if a file is being created, it can only contain additions
	if f.OldPosition == 0 && f.OldLines != 0 {
		return errors.New("file creation fragment contains context or deletion lines")
	}

	return nil
}

func lineCountErr(kind string, actual, reported int64) error {
	return fmt.Errorf("fragment contains %d %s lines but reports %d", actual, kind, reported)
}

// Line is a line in a text fragment.
type Line struct {
	Op   LineOp
	Line string
}

func (fl Line) String() string {
	return fl.Op.String() + fl.Line
}

// Old returns true if the line appears in the old content of the fragment.
func (fl Line) Old() bool {
	return fl.Op == OpContext || fl.Op == OpDelete
}

// New returns true if the line appears in the new content of the fragment.
func (fl Line) New() bool {
	return fl.Op == OpContext || fl.Op == OpAdd
}

// NoEOL returns true if the line is missing a trailing newline character.
func (fl Line) NoEOL() bool {
	return len(fl.Line) == 0 || fl.Line[len(fl.Line)-1] != '\n'
}

// LineOp describes the type of a text fragment line: context, added, or removed.
type LineOp int

const (
	// OpContext indicates a context line
	OpContext LineOp = iota
	// OpDelete indicates a deleted line
	OpDelete
	// OpAdd indicates an added line
	OpAdd
)

func (op LineOp) String() string {
	switch op {
	case OpContext:
		return " "
	case OpDelete:
		return "-"
	case OpAdd:
		return "+"
	}
	return "?"
}

// BinaryFragment describes changes to a binary file.
type BinaryFragment struct {
	Method BinaryPatchMethod
	Size   int64
	Data   []byte
}

// BinaryPatchMethod is the method used to create and apply the binary patch.
type BinaryPatchMethod int

const (
	// BinaryPatchDelta indicates the data uses Git's packfile encoding
	BinaryPatchDelta BinaryPatchMethod = iota
	// BinaryPatchLiteral indicates the data is the exact file content
	BinaryPatchLiteral
)
