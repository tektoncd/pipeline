//go:build !windows

package main

import (
	"bytes"
	"testing"
)

const fakeJWT = "eyJhbGciOiJSUzI1NiIsImtpZCI6InRlc3QifQ.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dGVzdHNpZ25hdHVyZQ"

func TestMaskingWriter(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no JWT present",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single JWT",
			input:    "TOKEN=" + fakeJWT + "!",
			expected: "TOKEN=***!",
		},
		{
			name:     "multiple JWTs",
			input:    "a=" + fakeJWT + " b=" + fakeJWT,
			expected: "a=*** b=***",
		},
		{
			name:     "set -x style output",
			input:    "+ TOKEN=" + fakeJWT + "\n+ echo done\n",
			expected: "+ TOKEN=***\n+ echo done\n",
		},
		{
			name:     "no false positive on partial eyJ",
			input:    "eyJust some text",
			expected: "eyJust some text",
		},
		{
			name:     "JWT at end of input (no trailing char)",
			input:    "TOKEN=" + fakeJWT,
			expected: "TOKEN=***",
		},
		{
			name:     "empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			w := newMaskingWriter(&buf)
			if _, err := w.Write([]byte(tc.input)); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if err := w.Flush(); err != nil {
				t.Fatalf("unexpected flush error: %v", err)
			}
			if got := buf.String(); got != tc.expected {
				t.Errorf("got %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestMaskingWriterSplitAcrossWrites(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf)

	// Split the JWT across two writes — the "eyJ" start is in write #1,
	// the rest comes in write #2
	jwt := fakeJWT
	splitPoint := 10 // splits in the middle of the header
	if _, err := w.Write([]byte("prefix " + jwt[:splitPoint])); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := w.Write([]byte(jwt[splitPoint:] + " suffix")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "prefix *** suffix"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterPartialPrefixHeldBack(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf)

	// Write ends with "ey" — should be held back in carry
	if _, err := w.Write([]byte("some text ey")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Next write completes it as NOT a JWT
	if _, err := w.Write([]byte("es are blue")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "some text eyes are blue"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterMultipleWritesNoJWT(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf)

	writes := []string{"hello ", "world ", "no secrets here\n"}
	for _, s := range writes {
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}

	if got, want := buf.String(), "hello world no secrets here\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestMaskingWriterFlushWithNoData(t *testing.T) {
	var buf bytes.Buffer
	w := newMaskingWriter(&buf)

	if err := w.Flush(); err != nil {
		t.Fatalf("unexpected flush error: %v", err)
	}
	if got := buf.String(); got != "" {
		t.Errorf("expected empty output, got %q", got)
	}
}
