package vt10x

import (
	expect "github.com/Netflix/go-expect"
	"github.com/kr/pty"
)

// NewVT10XConsole returns a new expect.Console that multiplexes the
// Stdin/Stdout to a VT10X terminal, allowing Console to interact with an
// application sending ANSI escape sequences.
func NewVT10XConsole(opts ...expect.ConsoleOpt) (*expect.Console, *State, error) {
	ptm, pts, err := pty.Open()
	if err != nil {
		return nil, nil, err
	}

	var state State
	term, err := Create(&state, pts)
	if err != nil {
		return nil, nil, err
	}

	c, err := expect.NewConsole(append(opts, expect.WithStdin(ptm), expect.WithStdout(term), expect.WithCloser(pts, ptm, term))...)
	if err != nil {
		return nil, nil, err
	}

	return c, &state, nil
}
