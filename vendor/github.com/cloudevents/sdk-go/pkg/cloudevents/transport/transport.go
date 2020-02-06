package transport

import (
	"context"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
)

// Transport is the interface for transport sender to send the converted Message
// over the underlying transport.
type Transport interface {
	Send(context.Context, cloudevents.Event) (context.Context, *cloudevents.Event, error)

	SetReceiver(Receiver)
	StartReceiver(context.Context) error

	// SetConverter sets the delegate to use for converting messages that have
	// failed to be decoded from known codecs for this transport.
	SetConverter(Converter)
	// HasConverter is true when a non-nil converter has been set.
	HasConverter() bool
}

// Receiver is an interface to define how a transport will invoke a listener
// of incoming events.
type Receiver interface {
	Receive(context.Context, cloudevents.Event, *cloudevents.EventResponse) error
}

// ReceiveFunc wraps a function as a Receiver object.
type ReceiveFunc func(ctx context.Context, e cloudevents.Event, er *cloudevents.EventResponse) error

// Receive implements Receiver.Receive
func (f ReceiveFunc) Receive(ctx context.Context, e cloudevents.Event, er *cloudevents.EventResponse) error {
	return f(ctx, e, er)
}

// Converter is an interface to define how a transport delegate to convert an
// non-understood transport message from the internal codecs. Providing a
// Converter allows incoming requests to be bridged to CloudEvents format if
// they have not been sent as an event in CloudEvents format.
type Converter interface {
	Convert(context.Context, Message, error) (*cloudevents.Event, error)
}
