package http

import (
	"bytes"
	"encoding/json"

	"io"
	"io/ioutil"
	"net/http"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
)

// type check that this transport message impl matches the contract
var _ transport.Message = (*Message)(nil)

// Message is an http transport message.
type Message struct {
	Header http.Header
	Body   []byte
}

// Response is an http transport response.
type Response struct {
	StatusCode int
	Message
}

// CloudEventsVersion inspects a message and tries to discover and return the
// CloudEvents spec version.
func (m Message) CloudEventsVersion() string {

	// TODO: the impl of this method needs to move into the codec.

	if m.Header != nil {
		// Try headers first.
		// v0.1, cased from the spec
		// Note: don't pass literal string direct to m.Header[] so that
		// go vet won't complain about non-canonical case.
		name := "CE-CloudEventsVersion"
		if v := m.Header[name]; len(v) == 1 {
			return v[0]
		}
		// v0.2, canonical casing
		if ver := m.Header.Get("CE-CloudEventsVersion"); ver != "" {
			return ver
		}

		// v0.2, cased from the spec
		name = "ce-specversion"
		if v := m.Header[name]; len(v) == 1 {
			return v[0]
		}
		// v0.2, canonical casing
		name = "ce-specversion"
		if ver := m.Header.Get(name); ver != "" {
			return ver
		}
	}

	// Then try the data body.
	// TODO: we need to use the correct decoding based on content type.

	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(m.Body, &raw); err != nil {
		return ""
	}

	// v0.1
	if v, ok := raw["cloudEventsVersion"]; ok {
		var version string
		if err := json.Unmarshal(v, &version); err != nil {
			return ""
		}
		return version
	}

	// v0.2
	if v, ok := raw["specversion"]; ok {
		var version string
		if err := json.Unmarshal(v, &version); err != nil {
			return ""
		}
		return version
	}

	return ""
}

func readAllClose(r io.ReadCloser) ([]byte, error) {
	if r != nil {
		defer r.Close()
		return ioutil.ReadAll(r)
	}
	return nil, nil
}

// NewMessage creates a new message from the Header and Body of
// an http.Request or http.Response
func NewMessage(header http.Header, body io.ReadCloser) (*Message, error) {
	var m Message
	err := m.Init(header, body)
	return &m, err
}

// NewResponse creates a new response from the Header and Body of
// an http.Request or http.Response
func NewResponse(header http.Header, body io.ReadCloser, statusCode int) (*Response, error) {
	resp := Response{StatusCode: statusCode}
	err := resp.Init(header, body)
	return &resp, err
}

// Copy copies a new Body and Header into a message, replacing any previous data.
func (m *Message) Init(header http.Header, body io.ReadCloser) error {
	m.Header = make(http.Header, len(header))
	copyHeadersEnsure(header, &m.Header)
	var err error
	m.Body, err = readAllClose(body)
	return err
}

func (m *Message) copyOut(header *http.Header, body *io.ReadCloser) {
	copyHeadersEnsure(m.Header, header)
	*body = nil
	if m.Body != nil {
		copy := append([]byte(nil), m.Body...)
		*body = ioutil.NopCloser(bytes.NewBuffer(copy))
	}
}

// ToRequest updates a http.Request from a Message.
// Replaces Body, ContentLength and Method, updates Headers.
// Panic if req is nil
func (m *Message) ToRequest(req *http.Request) {
	m.copyOut(&req.Header, &req.Body)
	req.ContentLength = int64(len(m.Body))
	req.Method = http.MethodPost
}

// ToResponse updates a http.Response from a Response.
// Replaces Body, updates Headers.
// Panic if resp is nil
func (m *Response) ToResponse(resp *http.Response) {
	m.copyOut(&resp.Header, &resp.Body)
	resp.ContentLength = int64(len(m.Body))
	resp.StatusCode = m.StatusCode
}
