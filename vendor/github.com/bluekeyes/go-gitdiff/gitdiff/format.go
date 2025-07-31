package gitdiff

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"strconv"
)

type formatter struct {
	w   io.Writer
	err error
}

func newFormatter(w io.Writer) *formatter {
	return &formatter{w: w}
}

func (fm *formatter) Write(p []byte) (int, error) {
	if fm.err != nil {
		return len(p), nil
	}
	if _, err := fm.w.Write(p); err != nil {
		fm.err = err
	}
	return len(p), nil
}

func (fm *formatter) WriteString(s string) (int, error) {
	fm.Write([]byte(s))
	return len(s), nil
}

func (fm *formatter) WriteByte(c byte) error {
	fm.Write([]byte{c})
	return nil
}

func (fm *formatter) WriteQuotedName(s string) {
	qpos := 0
	for i := 0; i < len(s); i++ {
		ch := s[i]
		if q, quoted := quoteByte(ch); quoted {
			if qpos == 0 {
				fm.WriteByte('"')
			}
			fm.WriteString(s[qpos:i])
			fm.Write(q)
			qpos = i + 1
		}
	}
	fm.WriteString(s[qpos:])
	if qpos > 0 {
		fm.WriteByte('"')
	}
}

var quoteEscapeTable = map[byte]byte{
	'\a': 'a',
	'\b': 'b',
	'\t': 't',
	'\n': 'n',
	'\v': 'v',
	'\f': 'f',
	'\r': 'r',
	'"':  '"',
	'\\': '\\',
}

func quoteByte(b byte) ([]byte, bool) {
	if q, ok := quoteEscapeTable[b]; ok {
		return []byte{'\\', q}, true
	}
	if b < 0x20 || b >= 0x7F {
		return []byte{
			'\\',
			'0' + (b>>6)&0o3,
			'0' + (b>>3)&0o7,
			'0' + (b>>0)&0o7,
		}, true
	}
	return nil, false
}

func (fm *formatter) FormatFile(f *File) {
	fm.WriteString("diff --git ")

	var aName, bName string
	switch {
	case f.OldName == "":
		aName = f.NewName
		bName = f.NewName

	case f.NewName == "":
		aName = f.OldName
		bName = f.OldName

	default:
		aName = f.OldName
		bName = f.NewName
	}

	fm.WriteQuotedName("a/" + aName)
	fm.WriteByte(' ')
	fm.WriteQuotedName("b/" + bName)
	fm.WriteByte('\n')

	if f.OldMode != 0 {
		if f.IsDelete {
			fmt.Fprintf(fm, "deleted file mode %o\n", f.OldMode)
		} else if f.NewMode != 0 {
			fmt.Fprintf(fm, "old mode %o\n", f.OldMode)
		}
	}

	if f.NewMode != 0 {
		if f.IsNew {
			fmt.Fprintf(fm, "new file mode %o\n", f.NewMode)
		} else if f.OldMode != 0 {
			fmt.Fprintf(fm, "new mode %o\n", f.NewMode)
		}
	}

	if f.Score > 0 {
		if f.IsCopy || f.IsRename {
			fmt.Fprintf(fm, "similarity index %d%%\n", f.Score)
		} else {
			fmt.Fprintf(fm, "dissimilarity index %d%%\n", f.Score)
		}
	}

	if f.IsCopy {
		if f.OldName != "" {
			fm.WriteString("copy from ")
			fm.WriteQuotedName(f.OldName)
			fm.WriteByte('\n')
		}
		if f.NewName != "" {
			fm.WriteString("copy to ")
			fm.WriteQuotedName(f.NewName)
			fm.WriteByte('\n')
		}
	}

	if f.IsRename {
		if f.OldName != "" {
			fm.WriteString("rename from ")
			fm.WriteQuotedName(f.OldName)
			fm.WriteByte('\n')
		}
		if f.NewName != "" {
			fm.WriteString("rename to ")
			fm.WriteQuotedName(f.NewName)
			fm.WriteByte('\n')
		}
	}

	if f.OldOIDPrefix != "" && f.NewOIDPrefix != "" {
		fmt.Fprintf(fm, "index %s..%s", f.OldOIDPrefix, f.NewOIDPrefix)

		// Mode is only included on the index line when it is not changing
		if f.OldMode != 0 && ((f.NewMode == 0 && !f.IsDelete) || f.OldMode == f.NewMode) {
			fmt.Fprintf(fm, " %o", f.OldMode)
		}

		fm.WriteByte('\n')
	}

	if f.IsBinary {
		if f.BinaryFragment == nil {
			fm.WriteString("Binary files ")
			fm.WriteQuotedName("a/" + aName)
			fm.WriteString(" and ")
			fm.WriteQuotedName("b/" + bName)
			fm.WriteString(" differ\n")
		} else {
			fm.WriteString("GIT binary patch\n")
			fm.FormatBinaryFragment(f.BinaryFragment)
			if f.ReverseBinaryFragment != nil {
				fm.FormatBinaryFragment(f.ReverseBinaryFragment)
			}
		}
	}

	// The "---" and "+++" lines only appear for text patches with fragments
	if len(f.TextFragments) > 0 {
		fm.WriteString("--- ")
		if f.OldName == "" {
			fm.WriteString("/dev/null")
		} else {
			fm.WriteQuotedName("a/" + f.OldName)
		}
		fm.WriteByte('\n')

		fm.WriteString("+++ ")
		if f.NewName == "" {
			fm.WriteString("/dev/null")
		} else {
			fm.WriteQuotedName("b/" + f.NewName)
		}
		fm.WriteByte('\n')

		for _, frag := range f.TextFragments {
			fm.FormatTextFragment(frag)
		}
	}
}

func (fm *formatter) FormatTextFragment(f *TextFragment) {
	fm.FormatTextFragmentHeader(f)
	fm.WriteByte('\n')

	for _, line := range f.Lines {
		fm.WriteString(line.Op.String())
		fm.WriteString(line.Line)
		if line.NoEOL() {
			fm.WriteString("\n\\ No newline at end of file\n")
		}
	}
}

func (fm *formatter) FormatTextFragmentHeader(f *TextFragment) {
	fmt.Fprintf(fm, "@@ -%d,%d +%d,%d @@", f.OldPosition, f.OldLines, f.NewPosition, f.NewLines)
	if f.Comment != "" {
		fm.WriteByte(' ')
		fm.WriteString(f.Comment)
	}
}

func (fm *formatter) FormatBinaryFragment(f *BinaryFragment) {
	const (
		maxBytesPerLine = 52
	)

	switch f.Method {
	case BinaryPatchDelta:
		fm.WriteString("delta ")
	case BinaryPatchLiteral:
		fm.WriteString("literal ")
	}
	fm.Write(strconv.AppendInt(nil, f.Size, 10))
	fm.WriteByte('\n')

	data := deflateBinaryChunk(f.Data)
	n := (len(data) / maxBytesPerLine) * maxBytesPerLine

	buf := make([]byte, base85Len(maxBytesPerLine))
	for i := 0; i < n; i += maxBytesPerLine {
		base85Encode(buf, data[i:i+maxBytesPerLine])
		fm.WriteByte('z')
		fm.Write(buf)
		fm.WriteByte('\n')
	}
	if remainder := len(data) - n; remainder > 0 {
		buf = buf[0:base85Len(remainder)]

		sizeChar := byte(remainder)
		if remainder <= 26 {
			sizeChar = 'A' + sizeChar - 1
		} else {
			sizeChar = 'a' + sizeChar - 27
		}

		base85Encode(buf, data[n:])
		fm.WriteByte(sizeChar)
		fm.Write(buf)
		fm.WriteByte('\n')
	}
	fm.WriteByte('\n')
}

func deflateBinaryChunk(data []byte) []byte {
	var b bytes.Buffer

	zw := zlib.NewWriter(&b)
	_, _ = zw.Write(data)
	_ = zw.Close()

	return b.Bytes()
}
