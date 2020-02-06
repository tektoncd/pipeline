package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
)

// CodecStructured represents an structured http transport codec for all versions.
// Intended to be used as a base class.
type CodecStructured struct {
	DefaultEncoding Encoding
}

func (v CodecStructured) encodeStructured(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	header := http.Header{}
	header.Set("Content-Type", cloudevents.ApplicationCloudEventsJSON)

	body, err := json.Marshal(e)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Header: header,
		Body:   body,
	}

	return msg, nil
}

func (v CodecStructured) decodeStructured(ctx context.Context, version string, msg transport.Message) (*cloudevents.Event, error) {
	m, ok := msg.(*Message)
	if !ok {
		return nil, fmt.Errorf("failed to convert transport.Message to http.Message")
	}
	event := cloudevents.New(version)
	err := json.Unmarshal(m.Body, &event)
	return &event, err
}
