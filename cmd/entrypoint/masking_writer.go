//go:build !windows

package main

import (
	"bytes"
	"io"
	"regexp"
	"strings"
)

const maskReplacement = "***"

type maskingWriter struct {
	underlying    io.Writer
	carry         []byte
	maskTokenByte []byte
	pattern       *regexp.Regexp
	exactSecrets  [][]byte
	// literalPrefixes holds the literal prefix of each regex pattern,
	// used by safeCutPoint to detect partial matches across writes.
	literalPrefixes [][]byte
	// maxSecretLen is the longest exact secret, used for safe-cut calculations.
	maxSecretLen int
}

// newMaskingWriterWithConfig creates a writer with configurable regex patterns
// and exact secret strings. If both are empty, no redaction occurs.
// Exact secrets are matched verbatim (no regex).
func newMaskingWriterWithConfig(w io.Writer, regexPatterns []string, exactSecrets []string) *maskingWriter {
	mw := &maskingWriter{
		underlying:    w,
		carry:         make([]byte, 0, 4096),
		maskTokenByte: []byte(maskReplacement),
	}

	mw.pattern, mw.literalPrefixes = buildCombinedPattern(regexPatterns)

	for _, s := range exactSecrets {
		s = strings.TrimSpace(s)
		if len(s) > 0 {
			mw.exactSecrets = append(mw.exactSecrets, []byte(s))
		}
	}
	for _, s := range mw.exactSecrets {
		if len(s) > mw.maxSecretLen {
			mw.maxSecretLen = len(s)
		}
	}

	return mw
}

// buildCombinedPattern compiles regex patterns into a single alternation and
// extracts the literal prefix of each pattern for safe-cut detection.
// Returns nil, nil when no valid patterns are provided.
func buildCombinedPattern(patterns []string) (*regexp.Regexp, [][]byte) {
	var valid []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if _, err := regexp.Compile(p); err == nil {
			valid = append(valid, p)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}
	var prefixes [][]byte
	for _, p := range valid {
		if lp := extractLiteralPrefix(p); len(lp) > 0 {
			prefixes = append(prefixes, []byte(lp))
		}
	}
	combined := strings.Join(valid, "|")
	return regexp.MustCompile(combined), prefixes
}

// extractLiteralPrefix returns the leading literal characters of a regex
// pattern — i.e. characters that are not regex metacharacters and always
// appear verbatim at the start of any match.
func extractLiteralPrefix(pattern string) string {
	var prefix strings.Builder
	for _, c := range []byte(pattern) {
		if strings.ContainsRune(`\.+*?^${}()|[]`, rune(c)) {
			break
		}
		prefix.WriteByte(c)
	}
	return prefix.String()
}

func (m *maskingWriter) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	// No masking configured — pass through without buffering.
	if len(m.exactSecrets) == 0 && m.pattern == nil {
		return m.underlying.Write(p)
	}

	m.carry = append(m.carry, p...)
	m.redactExactSecrets()

	if m.pattern == nil {
		safeEnd := m.safeCutPoint()
		if safeEnd > 0 {
			if _, err := m.underlying.Write(m.carry[:safeEnd]); err != nil {
				return len(p), err
			}
			m.carry = append(m.carry[:0], m.carry[safeEnd:]...)
		}
		return len(p), nil
	}

	loc := m.pattern.FindIndex(m.carry)
	if loc == nil {
		safeEnd := m.safeCutPoint()
		if safeEnd > 0 {
			if _, err := m.underlying.Write(m.carry[:safeEnd]); err != nil {
				return len(p), err
			}
			m.carry = append(m.carry[:0], m.carry[safeEnd:]...)
		}
		return len(p), nil
	}

	var out bytes.Buffer
	for loc != nil {
		out.Write(m.carry[:loc[0]])
		out.Write(m.maskTokenByte)
		m.carry = append(m.carry[:0], m.carry[loc[1]:]...)
		loc = m.pattern.FindIndex(m.carry)
	}
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

// redactExactSecrets replaces all occurrences of exact secret values in the
// carry buffer. This is done before regex matching so that secrets are caught
// even if they don't match a regex pattern.
func (m *maskingWriter) redactExactSecrets() {
	for _, secret := range m.exactSecrets {
		m.carry = bytes.ReplaceAll(m.carry, secret, m.maskTokenByte)
	}
}

// safeCutPoint determines how many bytes from the front of carry can be
// safely flushed. It holds back any trailing bytes that could be the start
// of a regex match or a partial exact secret.
func (m *maskingWriter) safeCutPoint() int {
	n := len(m.carry)
	if n == 0 {
		return 0
	}

	holdback := 1024
	if m.maxSecretLen-1 > holdback {
		holdback = m.maxSecretLen - 1
	}

	safe := 0
	if lookBackSafe := n - holdback; lookBackSafe > 0 {
		safe = lookBackSafe
	}

	for _, prefix := range m.literalPrefixes {
		for end := max(0, n-len(prefix)+1); end < n; end++ {
			tail := m.carry[end:]
			if bytes.HasPrefix(prefix, tail) && end < safe {
				safe = end
			}
		}
	}

	return safe
}

func (m *maskingWriter) Flush() error {
	if len(m.carry) == 0 {
		return nil
	}
	m.redactExactSecrets()
	out := m.carry
	if m.pattern != nil {
		out = m.pattern.ReplaceAll(m.carry, m.maskTokenByte)
	}
	_, err := m.underlying.Write(out)
	m.carry = m.carry[:0]
	return err
}
