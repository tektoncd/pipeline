package gitdiff

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/mail"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	mailHeaderPrefix   = "From "
	prettyHeaderPrefix = "commit "
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
// ParsePatchHeader makes no assumptions about the format of the patch title or
// message other than trimming whitespace and condensing blank lines. In
// particular, it does not remove the extra content that git format-patch adds
// to make emailed patches friendlier, like subject prefixes or commit stats.
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
		body := scanMessageBody(s, indent)
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

func scanMessageBody(s *bufio.Scanner, indent string) string {
	var b strings.Builder
	var empty int
	for i := 0; s.Scan(); i++ {
		line := s.Text()
		if strings.TrimSpace(line) == "" {
			empty++
			continue
		}

		if b.Len() > 0 {
			b.WriteByte('\n')
			if empty > 0 {
				b.WriteByte('\n')
			}
		}
		empty = 0

		line = strings.TrimRightFunc(line, unicode.IsSpace)
		line = strings.TrimPrefix(line, indent)
		b.WriteString(line)
	}
	return b.String()
}

func parseHeaderMail(mailLine string, r io.Reader) (*PatchHeader, error) {
	msg, err := mail.ReadMessage(r)
	if err != nil {
		return nil, err
	}

	h := &PatchHeader{}

	mailLine = mailLine[len(mailHeaderPrefix):]
	if i := strings.IndexByte(mailLine, ' '); i > 0 {
		h.SHA = mailLine[:i]
	}

	addrs, err := msg.Header.AddressList("From")
	if err != nil && !errors.Is(err, mail.ErrHeaderNotPresent) {
		return nil, err
	}
	if len(addrs) > 0 {
		addr := addrs[0]
		if addr.Name == "" {
			return nil, fmt.Errorf("invalid user string: %s", addr)
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

	h.Title = msg.Header.Get("Subject")

	s := bufio.NewScanner(msg.Body)
	h.Body = scanMessageBody(s, "")
	if s.Err() != nil {
		return nil, s.Err()
	}

	return h, nil
}
