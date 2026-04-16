//go:build !windows

package main

import (
	"bytes"
	"io"
	"regexp"
)

const maskReplacment = "***"

// JWT tokens always have the same structure:
// |______ header ______|.|______ payload ________|.|______ signature ______|
var jwtPattern = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)

type maskingWriter struct {
	underlying    io.Writer
	carry         []byte
	maskTokenByte []byte
}

func newMaskingWriter(w io.Writer) *maskingWriter {
	return &maskingWriter{
		underlying:    w,
		carry:         make([]byte, 0, 4096),
		maskTokenByte: []byte(maskReplacment),
	}
}

func (m *maskingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	m.carry = append(m.carry, p...)
	// Find all JWT matches in the carry buffer
	loc := jwtPattern.FindIndex(m.carry)
	if loc == nil {
		// No match found. Flush everything except a trailing portion
		// that could be the start of a JWT (starts with "eyJ").
		safeEnd := m.safeCutPoint()
		if safeEnd > 0 {
			if _, err := m.underlying.Write(m.carry[:safeEnd]); err != nil {
				return len(p), err
			}
			m.carry = append(m.carry[:0], m.carry[safeEnd:]...)
		}
		return len(p), nil
	}
	// Match found — flush everything before it, write the mask, keep the rest
	var out bytes.Buffer
	for loc != nil {
		out.Write(m.carry[:loc[0]])
		out.Write(m.maskTokenByte)
		m.carry = append(m.carry[:0], m.carry[loc[1]:]...)
		loc = jwtPattern.FindIndex(m.carry)
	}
	// Flush safe portion of remaining carry
	safeEnd := m.safeCutPoint()
	if safeEnd > 0 {
		out.Write(m.carry[:safeEnd])
		m.carry = append(m.carry[:0], m.carry[safeEnd:]...)
	}
	if out.Len() > 0 {
		if _, err := m.underlying.Write(out.Bytes()); err != nil {
			return len(p), err
		}
	}
	return len(p), nil
}

// safeCutPoint searches for the first appearance of "eyJ" or the prefix of it.
// Returns the index which up until it we can safely flush
func (m *maskingWriter) safeCutPoint() int {
	n := len(m.carry)
	if n == 0 {
		return 0
	}
	firstEyJ := bytes.Index(m.carry, []byte("eyJ"))
	if firstEyJ != -1 {
		return firstEyJ
	}
	for i := max(0, n-2); i < n; i++ {
		tail := string(m.carry[i:])
		if isJWTPrefix(tail) {
			return i
		}
	}
	return n
}

func isJWTPrefix(s string) bool {
	prefix := "eyJ"
	if len(s) > len(prefix) {
		return false
	}
	return prefix[:len(s)] == s
}

func (m *maskingWriter) Flush() error {
	if len(m.carry) == 0 {
		return nil
	}
	// On flush, redact any remaining matches and write everything
	result := jwtPattern.ReplaceAll(m.carry, m.maskTokenByte)
	_, err := m.underlying.Write(result)
	m.carry = m.carry[:0]
	return err
}
