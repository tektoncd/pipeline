// +build linux darwin dragonfly solaris openbsd netbsd freebsd

package vt10x

import (
	"bufio"
	"bytes"
	"io"
	"unicode"
	"unicode/utf8"
)

// VT represents the virtual terminal emulator.
type VT struct {
	dest *State
	rwc  io.ReadWriteCloser
	br   *bufio.Reader
}

// Create initializes a virtual terminal emulator with the target state
// and io.ReadWriteCloser input.
func Create(state *State, rwc io.ReadWriteCloser) (*VT, error) {
	t := &VT{
		dest: state,
		rwc:  rwc,
	}
	t.init()
	return t, nil
}

func (t *VT) init() {
	t.br = bufio.NewReader(t.rwc)
	t.dest.w = t.rwc
	t.dest.numlock = true
	t.dest.state = t.dest.parse
	t.dest.cur.attr.fg = DefaultFG
	t.dest.cur.attr.bg = DefaultBG
	t.Resize(80, 24)
	t.dest.reset()
}

// Write parses input and writes terminal changes to state.
func (t *VT) Write(p []byte) (int, error) {
	var written int
	r := bytes.NewReader(p)
	t.dest.lock()
	defer t.dest.unlock()
	for {
		c, sz, err := r.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			return written, err
		}
		written += sz
		if c == unicode.ReplacementChar && sz == 1 {
			if r.Len() == 0 {
				// not enough bytes for a full rune
				return written - 1, nil
			}
			t.dest.logln("invalid utf8 sequence")
			continue
		}
		t.dest.put(c)
	}
	return written, nil
}

// Close closes the io.ReadWriteCloser.
func (t *VT) Close() error {
	return t.rwc.Close()
}

// Parse blocks on read on pty or io.ReadCloser, then parses sequences until
// buffer empties. State is locked as soon as first rune is read, and unlocked
// when buffer is empty.
// TODO: add tests for expected blocking behavior
func (t *VT) Parse() error {
	var locked bool
	defer func() {
		if locked {
			t.dest.unlock()
		}
	}()
	for {
		c, sz, err := t.br.ReadRune()
		if err != nil {
			return err
		}
		if c == unicode.ReplacementChar && sz == 1 {
			t.dest.logln("invalid utf8 sequence")
			break
		}
		if !locked {
			t.dest.lock()
			locked = true
		}

		// put rune for parsing and update state
		t.dest.put(c)

		// break if our buffer is empty, or if buffer contains an
		// incomplete rune.
		n := t.br.Buffered()
		if n == 0 || (n < 4 && !fullRuneBuffered(t.br)) {
			break
		}
	}
	return nil
}

func fullRuneBuffered(br *bufio.Reader) bool {
	n := br.Buffered()
	buf, err := br.Peek(n)
	if err != nil {
		return false
	}
	return utf8.FullRune(buf)
}

// Resize reports new size to pty and updates state.
func (t *VT) Resize(cols, rows int) {
	t.dest.lock()
	defer t.dest.unlock()
	_ = t.dest.resize(cols, rows)
}
