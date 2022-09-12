package gitdiff

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"io"
	"io/ioutil"
	"strconv"
	"strings"
)

func (p *parser) ParseBinaryFragments(f *File) (n int, err error) {
	isBinary, hasData, err := p.ParseBinaryMarker()
	if err != nil || !isBinary {
		return 0, err
	}

	f.IsBinary = true
	if !hasData {
		return 0, nil
	}

	forward, err := p.ParseBinaryFragmentHeader()
	if err != nil {
		return 0, err
	}
	if forward == nil {
		return 0, p.Errorf(0, "missing data for binary patch")
	}
	if err := p.ParseBinaryChunk(forward); err != nil {
		return 0, err
	}
	f.BinaryFragment = forward

	// valid for reverse to not exist, but it must be valid if present
	reverse, err := p.ParseBinaryFragmentHeader()
	if err != nil {
		return 1, err
	}
	if reverse == nil {
		return 1, nil
	}
	if err := p.ParseBinaryChunk(reverse); err != nil {
		return 1, err
	}
	f.ReverseBinaryFragment = reverse

	return 1, nil
}

func (p *parser) ParseBinaryMarker() (isBinary bool, hasData bool, err error) {
	switch p.Line(0) {
	case "GIT binary patch\n":
		hasData = true
	case "Binary files differ\n":
	case "Files differ\n":
	default:
		return false, false, nil
	}

	if err = p.Next(); err != nil && err != io.EOF {
		return false, false, err
	}
	return true, hasData, nil
}

func (p *parser) ParseBinaryFragmentHeader() (*BinaryFragment, error) {
	parts := strings.SplitN(strings.TrimSuffix(p.Line(0), "\n"), " ", 2)
	if len(parts) < 2 {
		return nil, nil
	}

	frag := &BinaryFragment{}
	switch parts[0] {
	case "delta":
		frag.Method = BinaryPatchDelta
	case "literal":
		frag.Method = BinaryPatchLiteral
	default:
		return nil, nil
	}

	var err error
	if frag.Size, err = strconv.ParseInt(parts[1], 10, 64); err != nil {
		nerr := err.(*strconv.NumError)
		return nil, p.Errorf(0, "binary patch: invalid size: %v", nerr.Err)
	}

	if err := p.Next(); err != nil && err != io.EOF {
		return nil, err
	}
	return frag, nil
}

func (p *parser) ParseBinaryChunk(frag *BinaryFragment) error {
	// Binary fragments are encoded as a series of base85 encoded lines. Each
	// line starts with a character in [A-Za-z] giving the number of bytes on
	// the line, where A = 1 and z = 52, and ends with a newline character.
	//
	// The base85 encoding means each line is a multiple of 5 characters + 2
	// additional characters for the length byte and the newline. The fragment
	// ends with a blank line.
	const (
		shortestValidLine = "A00000\n"
		maxBytesPerLine   = 52
	)

	var data bytes.Buffer
	buf := make([]byte, maxBytesPerLine)
	for {
		line := p.Line(0)
		if line == "\n" {
			break
		}
		if len(line) < len(shortestValidLine) || (len(line)-2)%5 != 0 {
			return p.Errorf(0, "binary patch: corrupt data line")
		}

		byteCount, seq := int(line[0]), line[1:len(line)-1]
		switch {
		case 'A' <= byteCount && byteCount <= 'Z':
			byteCount = byteCount - 'A' + 1
		case 'a' <= byteCount && byteCount <= 'z':
			byteCount = byteCount - 'a' + 27
		default:
			return p.Errorf(0, "binary patch: invalid length byte")
		}

		// base85 encodes every 4 bytes into 5 characters, with up to 3 bytes of end padding
		maxByteCount := len(seq) / 5 * 4
		if byteCount > maxByteCount || byteCount < maxByteCount-3 {
			return p.Errorf(0, "binary patch: incorrect byte count")
		}

		if err := base85Decode(buf[:byteCount], []byte(seq)); err != nil {
			return p.Errorf(0, "binary patch: %v", err)
		}
		data.Write(buf[:byteCount])

		if err := p.Next(); err != nil {
			if err == io.EOF {
				return p.Errorf(0, "binary patch: unexpected EOF")
			}
			return err
		}
	}

	if err := inflateBinaryChunk(frag, &data); err != nil {
		return p.Errorf(0, "binary patch: %v", err)
	}

	// consume the empty line that ended the fragment
	if err := p.Next(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func inflateBinaryChunk(frag *BinaryFragment, r io.Reader) error {
	zr, err := zlib.NewReader(r)
	if err != nil {
		return err
	}

	data, err := ioutil.ReadAll(zr)
	if err != nil {
		return err
	}
	if err := zr.Close(); err != nil {
		return err
	}

	if int64(len(data)) != frag.Size {
		return fmt.Errorf("%d byte fragment inflated to %d", frag.Size, len(data))
	}
	frag.Data = data
	return nil
}
