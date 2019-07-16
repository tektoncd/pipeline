package transport

import (
	"context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
)

// Transport is the interface for transport sender to send the converted Message
// over the underlying transport.
type Transport interface {
	Send(context.Context, cloudevents.Event) (*cloudevents.Event, error)

	SetReceiver(Receiver)
	StartReceiver(context.Context) error
}

// Receiver is an interface to define how a transport will invoke a listener
// of incoming events.
type Receiver interface {
	Receive(context.Context, cloudevents.Event, *cloudevents.EventResponse) error
}
