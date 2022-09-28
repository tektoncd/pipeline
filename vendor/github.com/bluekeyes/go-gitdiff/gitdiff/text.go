package gitdiff

import (
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ParseTextFragments parses text fragments until the next file header or the
// end of the stream and attaches them to the given file. It returns the number
// of fragments that were added.
func (p *parser) ParseTextFragments(f *File) (n int, err error) {
	for {
		frag, err := p.ParseTextFragmentHeader()
		if err != nil {
			return n, err
		}
		if frag == nil {
			return n, nil
		}

		if f.IsNew && frag.OldLines > 0 {
			return n, p.Errorf(-1, "new file depends on old contents")
		}
		if f.IsDelete && frag.NewLines > 0 {
			return n, p.Errorf(-1, "deleted file still has contents")
		}

		if err := p.ParseTextChunk(frag); err != nil {
			return n, err
		}

		f.TextFragments = append(f.TextFragments, frag)
		n++
	}
}

func (p *parser) ParseTextFragmentHeader() (*TextFragment, error) {
	const (
		startMark = "@@ -"
		endMark   = " @@"
	)

	if !strings.HasPrefix(p.Line(0), startMark) {
		return nil, nil
	}

	parts := strings.SplitAfterN(p.Line(0), endMark, 2)
	if len(parts) < 2 {
		return nil, p.Errorf(0, "invalid fragment header")
	}

	f := &TextFragment{}
	f.Comment = strings.TrimSpace(parts[1])

	header := parts[0][len(startMark) : len(parts[0])-len(endMark)]
	ranges := strings.Split(header, " +")
	if len(ranges) != 2 {
		return nil, p.Errorf(0, "invalid fragment header")
	}

	var err error
	if f.OldPosition, f.OldLines, err = parseRange(ranges[0]); err != nil {
		return nil, p.Errorf(0, "invalid fragment header: %v", err)
	}
	if f.NewPosition, f.NewLines, err = parseRange(ranges[1]); err != nil {
		return nil, p.Errorf(0, "invalid fragment header: %v", err)
	}

	if err := p.Next(); err != nil && err != io.EOF {
		return nil, err
	}
	return f, nil
}

func (p *parser) ParseTextChunk(frag *TextFragment) error {
	if p.Line(0) == "" {
		return p.Errorf(0, "no content following fragment header")
	}

	oldLines, newLines := frag.OldLines, frag.NewLines
	for oldLines > 0 || newLines > 0 {
		line := p.Line(0)
		op, data := line[0], line[1:]

		switch op {
		case '\n':
			data = "\n"
			fallthrough // newer GNU diff versions create empty context lines
		case ' ':
			oldLines--
			newLines--
			if frag.LinesAdded == 0 && frag.LinesDeleted == 0 {
				frag.LeadingContext++
			} else {
				frag.TrailingContext++
			}
			frag.Lines = append(frag.Lines, Line{OpContext, data})
		case '-':
			oldLines--
			frag.LinesDeleted++
			frag.TrailingContext = 0
			frag.Lines = append(frag.Lines, Line{OpDelete, data})
		case '+':
			newLines--
			frag.LinesAdded++
			frag.TrailingContext = 0
			frag.Lines = append(frag.Lines, Line{OpAdd, data})
		case '\\':
			// this may appear in middle of fragment if it's for a deleted line
			if isNoNewlineMarker(line) {
				removeLastNewline(frag)
				break
			}
			fallthrough
		default:
			// TODO(bkeyes): if this is because we hit the next header, it
			// would be helpful to return the miscounts line error. We could
			// either test for the common headers ("@@ -", "diff --git") or
			// assume any invalid op ends the fragment; git returns the same
			// generic error in all cases so either is compatible
			return p.Errorf(0, "invalid line operation: %q", op)
		}

		if err := p.Next(); err != nil {
			if err == io.EOF {
				break
			}
			return err
		}
	}

	if oldLines != 0 || newLines != 0 {
		hdr := max(frag.OldLines-oldLines, frag.NewLines-newLines) + 1
		return p.Errorf(-hdr, "fragment header miscounts lines: %+d old, %+d new", -oldLines, -newLines)
	}
	if frag.LinesAdded == 0 && frag.LinesDeleted == 0 {
		return p.Errorf(0, "fragment contains no changes")
	}

	// check for a final "no newline" marker since it is not included in the
	// counters used to stop the loop above
	if isNoNewlineMarker(p.Line(0)) {
		removeLastNewline(frag)
		if err := p.Next(); err != nil && err != io.EOF {
			return err
		}
	}

	return nil
}

func isNoNewlineMarker(s string) bool {
	// test for "\ No newline at end of file" by prefix because the text
	// changes by locale (git claims all versions are at least 12 chars)
	return len(s) >= 12 && s[:2] == "\\ "
}

func removeLastNewline(frag *TextFragment) {
	if len(frag.Lines) > 0 {
		last := &frag.Lines[len(frag.Lines)-1]
		last.Line = strings.TrimSuffix(last.Line, "\n")
	}
}

func parseRange(s string) (start int64, end int64, err error) {
	parts := strings.SplitN(s, ",", 2)

	if start, err = strconv.ParseInt(parts[0], 10, 64); err != nil {
		nerr := err.(*strconv.NumError)
		return 0, 0, fmt.Errorf("bad start of range: %s: %v", parts[0], nerr.Err)
	}

	if len(parts) > 1 {
		if end, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
			nerr := err.(*strconv.NumError)
			return 0, 0, fmt.Errorf("bad end of range: %s: %v", parts[1], nerr.Err)
		}
	} else {
		end = 1
	}

	return
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
