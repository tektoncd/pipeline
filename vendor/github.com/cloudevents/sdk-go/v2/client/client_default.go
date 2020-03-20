package client

import (
	"github.com/cloudevents/sdk-go/v2/protocol/http"
)

// NewDefault provides the good defaults for the common case using an HTTP
// Protocol client. The http transport has had WithBinaryEncoding http
// transport option applied to it. The client will always send Binary
// encoding but will inspect the outbound event context and match the version.
// The WithTimeNow, and WithUUIDs client options are also applied to the
// client, all outbound events will have a time and id set if not already
// present.
func NewDefault() (Client, error) {
	p, err := http.New()
	if err != nil {
		return nil, err
	}

	c, err := NewObserved(p, WithTimeNow(), WithUUIDs())
	if err != nil {
		return nil, err
	}

	return c, nil
}
