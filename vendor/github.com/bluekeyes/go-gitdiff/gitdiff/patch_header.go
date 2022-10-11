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

// A PatchHeaderOption modifies the behavior of ParsePatchHeader.
type PatchHeaderOption func(*patchHeaderOptions)

// SubjectCleanMode controls how ParsePatchHeader cleans subject lines when
// parsing mail-formatted patches.
type SubjectCleanMode int

const (
	// SubjectCleanWhitespace removes leading and trailing whitespace.
	SubjectCleanWhitespace SubjectCleanMode = iota

	// SubjectCleanAll removes leading and trailing whitespace, leading "Re:",
	// "re:", and ":" strings, and leading strings enclosed by '[' and ']'.
	// This is the default behavior of git (see `git mailinfo`) and this
	// package.
	SubjectCleanAll

	// SubjectCleanPatchOnly is the same as SubjectCleanAll, but only removes
	// leading strings enclosed by '[' and ']' if they start with "PATCH".
	SubjectCleanPatchOnly
)

// WithSubjectCleanMode sets the SubjectCleanMode for header parsing. By
// default, uses SubjectCleanAll.
func WithSubjectCleanMode(m SubjectCleanMode) PatchHeaderOption {
	return func(opts *patchHeaderOptions) {
		opts.subjectCleanMode = m
	}
}

type patchHeaderOptions struct {
	subjectCleanMode SubjectCleanMode
}

// ParsePatchHeader parses the preamble string returned by [Parse] into a
// PatchHeader. Due to the variety of header formats, some fields of the parsed
// PatchHeader may be unset after parsing.
//
// Supported formats are the short, medium, full, fuller, and email pretty
// formats used by `git diff`, `git log`, and `git show` and the UNIX mailbox
// format used by `git format-patch`.
//
// When parsing mail-formatted headers, ParsePatchHeader tries to remove
// email-specific content from the title and body:
//
//   - Based on the SubjectCleanMode, remove prefixes like reply markers and
//     "[PATCH]" strings from the subject, saving any removed content in the
//     SubjectPrefix field. Parsing always discards leading and trailing
//     whitespace from the subject line. The default mode is SubjectCleanAll.
//
//   - If the body contains a "---" line (3 hyphens), remove that line and any
//     content after it from the body and save it in the BodyAppendix field.
//
// ParsePatchHeader tries to process content it does not understand wthout
// returning errors, but will return errors if well-identified content like
// dates or identies uses unknown or invalid formats.
func ParsePatchHeader(header string, options ...PatchHeaderOption) (*PatchHeader, error) {
	opts := patchHeaderOptions{
		subjectCleanMode: SubjectCleanAll, // match git defaults
	}
	for _, optFn := range options {
		optFn(&opts)
	}

	header = strings.TrimSpace(header)
	if header == "" {
		return &PatchHeader{}, nil
	}

	var firstLine, rest string
	if idx := strings.IndexByte(header, '\n'); idx >= 0 {
		firstLine = header[:idx]
		rest = header[idx+1:]
	} else {
		firstLine = header
		rest = ""
	}

	switch {
	case strings.HasPrefix(firstLine, mailHeaderPrefix):
		return parseHeaderMail(firstLine, strings.NewReader(rest), opts)

	case strings.HasPrefix(firstLine, mailMinimumHeaderPrefix):
		// With a minimum header, the first line is part of the actual mail
		// content and needs to be parsed as part of the "rest"
		return parseHeaderMail("", strings.NewReader(header), opts)

	case strings.HasPrefix(firstLine, prettyHeaderPrefix):
		return parseHeaderPretty(firstLine, strings.NewReader(rest))
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

	prettyLine = strings.TrimPrefix(prettyLine, prettyHeaderPrefix)
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
		// Don't check for an appendix, pretty headers do not contain them
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

func parseHeaderMail(mailLine string, r io.Reader, opts patchHeaderOptions) (*PatchHeader, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, err
	}

	h := &PatchHeader{}

	if strings.HasPrefix(mailLine, mailHeaderPrefix) {
		mailLine = strings.TrimPrefix(mailLine, mailHeaderPrefix)
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
	h.SubjectPrefix, h.Title = cleanSubject(subject, opts.subjectCleanMode)

	s := bufio.NewScanner(msg.Body)
	h.Body, h.BodyAppendix = scanMessageBody(s, "", true)
	if s.Err() != nil {
		return nil, s.Err()
	}

	return h, nil
}

func cleanSubject(s string, mode SubjectCleanMode) (prefix string, subject string) {
	switch mode {
	case SubjectCleanAll, SubjectCleanPatchOnly:
	case SubjectCleanWhitespace:
		return "", strings.TrimSpace(decodeSubject(s))
	default:
		panic(fmt.Sprintf("unknown clean mode: %d", mode))
	}

	// Based on the algorithm from Git in mailinfo.c:cleanup_subject()
	// If compatibility with `git am` drifts, go there to see if there are any updates.

	at := 0
	for at < len(s) {
		switch s[at] {
		case 'r', 'R':
			// Detect re:, Re:, rE: and RE:
			if at+2 < len(s) && (s[at+1] == 'e' || s[at+1] == 'E') && s[at+2] == ':' {
				at += 3
				continue
			}

		case ' ', '\t', ':':
			// Delete whitespace and duplicate ':' characters
			at++
			continue

		case '[':
			if i := strings.IndexByte(s[at:], ']'); i > 0 {
				if mode == SubjectCleanAll || strings.Contains(s[at:at+i+1], "PATCH") {
					at += i + 1
					continue
				}
			}
		}

		// Nothing was removed, end processing
		break
	}

	prefix = strings.TrimLeftFunc(s[:at], unicode.IsSpace)
	subject = strings.TrimRightFunc(decodeSubject(s[at:]), unicode.IsSpace)
	return
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
