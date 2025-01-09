package gitdiff

import (
	"fmt"
	"strings"
)

// PatchIdentity identifies a person who authored or committed a patch.
type PatchIdentity struct {
	Name  string
	Email string
}

func (i PatchIdentity) String() string {
	name := i.Name
	if name == "" {
		name = `""`
	}
	return fmt.Sprintf("%s <%s>", name, i.Email)
}

// ParsePatchIdentity parses a patch identity string. A patch identity contains
// an email address and an optional name in [RFC 5322] format. This is either a
// plain email adddress or a name followed by an address in angle brackets:
//
//	author@example.com
//	Author Name <author@example.com>
//
// If the input is not one of these formats, ParsePatchIdentity applies a
// heuristic to separate the name and email portions. If both the name and
// email are missing or empty, ParsePatchIdentity returns an error. It
// otherwise does not validate the result.
//
// [RFC 5322]: https://datatracker.ietf.org/doc/html/rfc5322
func ParsePatchIdentity(s string) (PatchIdentity, error) {
	s = normalizeSpace(s)
	s = unquotePairs(s)

	var name, email string
	if at := strings.IndexByte(s, '@'); at >= 0 {
		start, end := at, at
		for start >= 0 && !isRFC5332Space(s[start]) && s[start] != '<' {
			start--
		}
		for end < len(s) && !isRFC5332Space(s[end]) && s[end] != '>' {
			end++
		}
		email = s[start+1 : end]

		// Adjust the boundaries so that we drop angle brackets, but keep
		// spaces when removing the email to form the name.
		if start < 0 || s[start] != '<' {
			start++
		}
		if end >= len(s) || s[end] != '>' {
			end--
		}
		name = s[:start] + s[end+1:]
	} else {
		start, end := 0, 0
		for i := 0; i < len(s); i++ {
			if s[i] == '<' && start == 0 {
				start = i + 1
			}
			if s[i] == '>' && start > 0 {
				end = i
				break
			}
		}
		if start > 0 && end >= start {
			email = strings.TrimSpace(s[start:end])
			name = s[:start-1]
		}
	}

	// After extracting the email, the name might contain extra whitespace
	// again and may be surrounded by comment characters. The git source gives
	// these examples of when this can happen:
	//
	//   "Name <email@domain>"
	//   "email@domain (Name)"
	//   "Name <email@domain> (Comment)"
	//
	name = normalizeSpace(name)
	if strings.HasPrefix(name, "(") && strings.HasSuffix(name, ")") {
		name = name[1 : len(name)-1]
	}
	name = strings.TrimSpace(name)

	// If the name is empty or contains email-like characters, use the email
	// instead (assuming one exists)
	if name == "" || strings.ContainsAny(name, "@<>") {
		name = email
	}

	if name == "" && email == "" {
		return PatchIdentity{}, fmt.Errorf("invalid identity string %q", s)
	}
	return PatchIdentity{Name: name, Email: email}, nil
}

// unquotePairs process the RFC5322 tokens "quoted-string" and "comment" to
// remove any "quoted-pairs" (backslash-espaced characters). It also removes
// the quotes from any quoted strings, but leaves the comment delimiters.
func unquotePairs(s string) string {
	quote := false
	comments := 0
	escaped := false

	var out strings.Builder
	for i := 0; i < len(s); i++ {
		if escaped {
			escaped = false
		} else {
			switch s[i] {
			case '\\':
				// quoted-pair is only allowed in quoted-string/comment
				if quote || comments > 0 {
					escaped = true
					continue // drop '\' character
				}

			case '"':
				if comments == 0 {
					quote = !quote
					continue // drop '"' character
				}

			case '(':
				if !quote {
					comments++
				}
			case ')':
				if comments > 0 {
					comments--
				}
			}
		}
		out.WriteByte(s[i])
	}
	return out.String()
}

// normalizeSpace trims leading and trailing whitespace from s and converts
// inner sequences of one or more whitespace characters to single spaces.
func normalizeSpace(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !isRFC5332Space(c) {
			if sb.Len() > 0 && isRFC5332Space(s[i-1]) {
				sb.WriteByte(' ')
			}
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

func isRFC5332Space(c byte) bool {
	switch c {
	case '\t', '\n', '\r', ' ':
		return true
	}
	return false
}
