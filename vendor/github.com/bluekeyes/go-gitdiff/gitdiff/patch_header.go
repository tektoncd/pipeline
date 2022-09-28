package gitdiff

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/quotedprintable"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	mailHeaderPrefix        = "From "
	prettyHeaderPrefix      = "commit "
	mailMinimumHeaderPrefix = "From:"
)

// PatchHeader is a parsed version of the preamble content that appears before
// the first diff in a patch. It includes metadata about the patch, such as the
// author and a subject.
type PatchHeader struct {
	// The SHA of the commit the patch was generated from. Empty if the SHA is
	// not included in the header.
	SHA string

	// The author details of the patch. If these details are not included in
	// the header, Author is nil and AuthorDate is the zero time.
	Author     *PatchIdentity
	AuthorDate time.Time

	// The committer details of the patch. If these details are not included in
	// the header, Committer is nil and CommitterDate is the zero time.
	Committer     *PatchIdentity
	CommitterDate time.Time

	// The title and body of the commit message describing the changes in the
	// patch. Empty if no message is included in the header.
	Title string
	Body  string

	// If the preamble looks like an email, ParsePatchHeader will
	// remove prefixes such as `Re: ` and `[PATCH v3 5/17]` from the
	// Title and place them here.
	SubjectPrefix string

	// If the preamble looks like an email, and it contains a `---`
	// line, that line will be removed and everything after it will be
	// placed in BodyAppendix.
	BodyAppendix string
}

// Message returns the commit message for the header. The message consists of
// the title and the body separated by an empty line.
func (h *PatchHeader) Message() string {
	var msg strings.Builder
	if h != nil {
		msg.WriteString(h.Title)
		if h.Body != "" {
			msg.WriteString("\n\n")
			msg.WriteString(h.Body)
		}
	}
	return msg.String()
}

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

// ParsePatchIdentity parses a patch identity string. A valid string contains a
// non-empty name followed by an email address in angle brackets. Like Git,
// ParsePatchIdentity does not require that the email address is valid or
// properly formatted, only that it is non-empty. The name must not contain a
// left angle bracket, '<', and the email address must not contain a right
// angle bracket, '>'.
func ParsePatchIdentity(s string) (PatchIdentity, error) {
	var emailStart, emailEnd int
	for i, c := range s {
		if c == '<' && emailStart == 0 {
			emailStart = i + 1
		}
		if c == '>' && emailStart > 0 {
			emailEnd = i
			break
		}
	}
	if emailStart > 0 && emailEnd == 0 {
		return PatchIdentity{}, fmt.Errorf("invalid identity string: unclosed email section: %s", s)
	}

	var name, email string
	if emailStart > 0 {
		name = strings.TrimSpace(s[:emailStart-1])
	}
	if emailStart > 0 && emailEnd > 0 {
		email = strings.TrimSpace(s[emailStart:emailEnd])
	}
	if name == "" || email == "" {
		return PatchIdentity{}, fmt.Errorf("invalid identity string: %s", s)
	}

	return PatchIdentity{Name: name, Email: email}, nil
}

// ParsePatchDate parses a patch date string. It returns the parsed time or an
// error if s has an unknown format. ParsePatchDate supports the iso, rfc,
// short, raw, unix, and default formats (with local variants) used by the
// --date flag in Git.
func ParsePatchDate(s string) (time.Time, error) {
	const (
		isoFormat          = "2006-01-02 15:04:05 -0700"
		isoStrictFormat    = "2006-01-02T15:04:05-07:00"
		rfc2822Format      = "Mon, 2 Jan 2006 15:04:05 -0700"
		shortFormat        = "2006-01-02"
		defaultFormat      = "Mon Jan 2 15:04:05 2006 -0700"
		defaultLocalFormat = "Mon Jan 2 15:04:05 2006"
	)

	if s == "" {
		return time.Time{}, nil
	}

	for _, fmt := range []string{
		isoFormat,
		isoStrictFormat,
		rfc2822Format,
		shortFormat,
		defaultFormat,
		defaultLocalFormat,
	} {
		if t, err := time.ParseInLocation(fmt, s, time.Local); err == nil {
			return t, nil
		}
	}

	// unix format
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(unix, 0), nil
	}

	// raw format
	if space := strings.IndexByte(s, ' '); space > 0 {
		unix, uerr := strconv.ParseInt(s[:space], 10, 64)
		zone, zerr := time.Parse("-0700", s[space+1:])
		if uerr == nil && zerr == nil {
			return time.Unix(unix, 0).In(zone.Location()), nil
		}
	}

	return time.Time{}, fmt.Errorf("unknown date format: %s", s)
}

// ParsePatchHeader parses a preamble string as returned by Parse into a
// PatchHeader. Due to the variety of header formats, some fields of the parsed
// PatchHeader may be unset after parsing.
//
// Supported formats are the short, medium, full, fuller, and email pretty
// formats used by git diff, git log, and git show and the UNIX mailbox format
// used by git format-patch.
//
// If ParsePatchHeader detects that it is handling an email, it will
// remove extra content at the beginning of the title line, such as
// `[PATCH]` or `Re:` in the same way that `git mailinfo` does.
// SubjectPrefix will be set to the value of this removed string.
// (`git mailinfo` is the core part of `git am` that pulls information
// out of an individual mail.)
//
// Additionally, if ParsePatchHeader detects that it's handling an
// email, it will remove a `---` line and put anything after it into
// BodyAppendix.
//
// Those wishing the effect of a plain `git am` should use
// `PatchHeader.Title + "\n" + PatchHeader.Body` (or
// `PatchHeader.Message()`).  Those wishing to retain the subject
// prefix and appendix material should use `PatchHeader.SubjectPrefix
// + PatchHeader.Title + "\n" + PatchHeader.Body + "\n" +
// PatchHeader.BodyAppendix`.
func ParsePatchHeader(s string) (*PatchHeader, error) {
	r := bufio.NewReader(strings.NewReader(s))

	var line string
	for {
		var err error
		line, err = r.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)
		if len(line) > 0 {
			break
		}
	}

	switch {
	case strings.HasPrefix(line, mailHeaderPrefix):
		return parseHeaderMail(line, r)
	case strings.HasPrefix(line, mailMinimumHeaderPrefix):
		r = bufio.NewReader(strings.NewReader(s))
		return parseHeaderMail("", r)
	case strings.HasPrefix(line, prettyHeaderPrefix):
		return parseHeaderPretty(line, r)
	}
	return nil, errors.New("unrecognized patch header format")
}

func parseHeaderPretty(prettyLine string, r io.Reader) (*PatchHeader, error) {
	const (
		authorPrefix     = "Author:"
		commitPrefix     = "Commit:"
		datePrefix       = "Date:"
		authorDatePrefix = "AuthorDate:"
		commitDatePrefix = "CommitDate:"
	)

	h := &PatchHeader{}

	prettyLine = prettyLine[len(prettyHeaderPrefix):]
	if i := strings.IndexByte(prettyLine, ' '); i > 0 {
		h.SHA = prettyLine[:i]
	} else {
		h.SHA = prettyLine
	}

	s := bufio.NewScanner(r)
	for s.Scan() {
		line := s.Text()

		// empty line marks end of fields, remaining lines are title/message
		if strings.TrimSpace(line) == "" {
			break
		}

		switch {
		case strings.HasPrefix(line, authorPrefix):
			u, err := ParsePatchIdentity(line[len(authorPrefix):])
			if err != nil {
				return nil, err
			}
			h.Author = &u

		case strings.HasPrefix(line, commitPrefix):
			u, err := ParsePatchIdentity(line[len(commitPrefix):])
			if err != nil {
				return nil, err
			}
			h.Committer = &u

		case strings.HasPrefix(line, datePrefix):
			d, err := ParsePatchDate(strings.TrimSpace(line[len(datePrefix):]))
			if err != nil {
				return nil, err
			}
			h.AuthorDate = d

		case strings.HasPrefix(line, authorDatePrefix):
			d, err := ParsePatchDate(strings.TrimSpace(line[len(authorDatePrefix):]))
			if err != nil {
				return nil, err
			}
			h.AuthorDate = d

		case strings.HasPrefix(line, commitDatePrefix):
			d, err := ParsePatchDate(strings.TrimSpace(line[len(commitDatePrefix):]))
			if err != nil {
				return nil, err
			}
			h.CommitterDate = d
		}
	}
	if s.Err() != nil {
		return nil, s.Err()
	}

	title, indent := scanMessageTitle(s)
	if s.Err() != nil {
		return nil, s.Err()
	}
	h.Title = title

	if title != "" {
		// Don't check for an appendix
		body, _ := scanMessageBody(s, indent, false)
		if s.Err() != nil {
			return nil, s.Err()
		}
		h.Body = body
	}

	return h, nil
}

func scanMessageTitle(s *bufio.Scanner) (title string, indent string) {
	var b strings.Builder
	for i := 0; s.Scan(); i++ {
		line := s.Text()
		trimLine := strings.TrimSpace(line)
		if trimLine == "" {
			break
		}

		if i == 0 {
			if start := strings.IndexFunc(line, func(c rune) bool { return !unicode.IsSpace(c) }); start > 0 {
				indent = line[:start]
			}
		}
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(trimLine)
	}
	return b.String(), indent
}

func scanMessageBody(s *bufio.Scanner, indent string, separateAppendix bool) (string, string) {
	// Body and appendix
	var body, appendix strings.Builder
	c := &body
	var empty int
	for i := 0; s.Scan(); i++ {
		line := s.Text()

		line = strings.TrimRightFunc(line, unicode.IsSpace)
		line = strings.TrimPrefix(line, indent)

		if line == "" {
			empty++
			continue
		}

		// If requested, parse out "appendix" information (often added
		// by `git format-patch` and removed by `git am`).
		if separateAppendix && c == &body && line == "---" {
			c = &appendix
			continue
		}

		if c.Len() > 0 {
			c.WriteByte('\n')
			if empty > 0 {
				c.WriteByte('\n')
			}
		}
		empty = 0

		c.WriteString(line)
	}
	return body.String(), appendix.String()
}

func parseHeaderMail(mailLine string, r io.Reader) (*PatchHeader, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, err
	}

	h := &PatchHeader{}

	if len(mailLine) > len(mailHeaderPrefix) {
		mailLine = mailLine[len(mailHeaderPrefix):]
		if i := strings.IndexByte(mailLine, ' '); i > 0 {
			h.SHA = mailLine[:i]
		}
	}

	addrs, err := msg.Header.AddressList("From")
	if err != nil && !errors.Is(err, mail.ErrHeaderNotPresent) {
		return nil, err
	}
	if len(addrs) > 0 {
		addr := addrs[0]
		if addr.Name == "" {
			addr.Name = addr.Address
		}
		h.Author = &PatchIdentity{Name: addr.Name, Email: addr.Address}
	}

	date := msg.Header.Get("Date")
	if date != "" {
		d, err := ParsePatchDate(date)
		if err != nil {
			return nil, err
		}
		h.AuthorDate = d
	}

	subject := msg.Header.Get("Subject")
	h.SubjectPrefix, h.Title = parseSubject(subject)

	s := bufio.NewScanner(msg.Body)
	h.Body, h.BodyAppendix = scanMessageBody(s, "", true)
	if s.Err() != nil {
		return nil, s.Err()
	}

	return h, nil
}

// Takes an email subject and returns the patch prefix and commit
// title.  i.e., `[PATCH v3 3/5] Implement foo` would return `[PATCH
// v3 3/5] ` and `Implement foo`
func parseSubject(s string) (string, string) {
	// This is meant to be compatible with
	// https://github.com/git/git/blob/master/mailinfo.c:cleanup_subject().
	// If compatibility with `git am` drifts, go there to see if there
	// are any updates.

	at := 0
	for at < len(s) {
		switch s[at] {
		case 'r', 'R':
			// Detect re:, Re:, rE: and RE:
			if at+2 < len(s) &&
				(s[at+1] == 'e' || s[at+1] == 'E') &&
				s[at+2] == ':' {
				at += 3
				continue
			}

		case ' ', '\t', ':':
			// Delete whitespace and duplicate ':' characters
			at++
			continue

		case '[':
			// Look for closing parenthesis
			j := at + 1
			for ; j < len(s); j++ {
				if s[j] == ']' {
					break
				}
			}

			if j < len(s) {
				at = j + 1
				continue
			}
		}

		// Only loop if we actually removed something
		break
	}

	return s[:at], decodeSubject(s[at:])
}

// Decodes a subject line. Currently only supports quoted-printable UTF-8. This format is the result
// of a `git format-patch` when the commit title has a non-ASCII character (i.e. an emoji).
// See for reference: https://stackoverflow.com/questions/27695749/gmail-api-not-respecting-utf-encoding-in-subject
func decodeSubject(encoded string) string {
	if !strings.HasPrefix(encoded, "=?UTF-8?q?") {
		// not UTF-8 encoded
		return encoded
	}

	// If the subject is too long, `git format-patch` may produce a subject line across
	// multiple lines. When parsed, this can look like the following:
	// <UTF8-prefix><first-line> <UTF8-prefix><second-line>
	payload := " " + encoded
	payload = strings.ReplaceAll(payload, " =?UTF-8?q?", "")
	payload = strings.ReplaceAll(payload, "?=", "")

	decoded, err := ioutil.ReadAll(quotedprintable.NewReader(strings.NewReader(payload)))
	if err != nil {
		// if err, abort decoding and return original subject
		return encoded
	}

	return string(decoded)
}
