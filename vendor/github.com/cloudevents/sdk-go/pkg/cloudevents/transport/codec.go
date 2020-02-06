package transport

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
)

// Codec is the interface for transport codecs to convert between transport
// specific payloads and the Message interface.
type Codec interface {
	Encode(context.Context, cloudevents.Event) (Message, error)
	Decode(context.Context, Message) (*cloudevents.Event, error)
}

// ErrMessageEncodingUnknown is an error produced when the encoding for an incoming
// message can not be understood.
type ErrMessageEncodingUnknown struct {
	codec     string
	transport string
}

// NewErrMessageEncodingUnknown makes a new ErrMessageEncodingUnknown.
func NewErrMessageEncodingUnknown(codec, transport string) *ErrMessageEncodingUnknown {
	return &ErrMessageEncodingUnknown{
		codec:     codec,
		transport: transport,
	}
}

// Error implements error.Error
func (e *ErrMessageEncodingUnknown) Error() string {
	return fmt.Sprintf("message encoding unknown for %s codec on %s transport", e.codec, e.transport)
}
