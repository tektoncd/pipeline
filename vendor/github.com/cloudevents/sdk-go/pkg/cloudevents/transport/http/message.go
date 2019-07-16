package http

import (
	"encoding/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"net/http"
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
	Header     http.Header
	Body       []byte
}

// CloudEventsVersion inspects a message and tries to discover and return the
// CloudEvents spec version.
func (m Message) CloudEventsVersion() string {

	// TODO: the impl of this method needs to move into the codec.

	if m.Header != nil {
		// Try headers first.
		// v0.1, cased from the spec
		if v := m.Header["CE-CloudEventsVersion"]; len(v) == 1 {
			return v[0]
		}
		// v0.2, canonical casing
		if ver := m.Header.Get("CE-CloudEventsVersion"); ver != "" {
			return ver
		}

		// v0.2, cased from the spec
		if v := m.Header["ce-specversion"]; len(v) == 1 {
			return v[0]
		}
		// v0.2, canonical casing
		if ver := m.Header.Get("ce-specversion"); ver != "" {
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
