package client

import (
	"context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/extensions"
	"github.com/cloudevents/sdk-go/v2/observability"
	"go.opencensus.io/trace"
)

// New produces a new client with the provided transport object and applied
// client options.
func NewObserved(protocol interface{}, opts ...Option) (Client, error) {
	client, err := New(protocol, opts...)
	if err != nil {
		return nil, err
	}

	c := &obsClient{client: client}

	if err := c.applyOptions(opts...); err != nil {
		return nil, err
	}
	return c, nil
}

type obsClient struct {
	client Client

	addTracing bool
}

func (c *obsClient) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(c); err != nil {
			return err
		}
	}
	return nil
}

// Send transmits the provided event on a preconfigured Protocol. Send returns
// an error if there was an an issue validating the outbound event or the
// transport returns an error.
func (c *obsClient) Send(ctx context.Context, e event.Event) error {
	ctx, r := observability.NewReporter(ctx, reportSend)
	ctx, span := trace.StartSpan(ctx, observability.ClientSpanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(EventTraceAttributes(&e)...)
	}

	if c.addTracing {
		e.Context = e.Context.Clone()
		extensions.FromSpanContext(span.SpanContext()).AddTracingAttributes(&e)
	}

	err := c.client.Send(ctx, e)

	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}

func (c *obsClient) Request(ctx context.Context, e event.Event) (*event.Event, error) {
	ctx, r := observability.NewReporter(ctx, reportRequest)
	ctx, span := trace.StartSpan(ctx, observability.ClientSpanName, trace.WithSpanKind(trace.SpanKindClient))
	defer span.End()
	if span.IsRecordingEvents() {
		span.AddAttributes(EventTraceAttributes(&e)...)
	}

	resp, err := c.client.Request(ctx, e)

	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return resp, err
}

// StartReceiver sets up the given fn to handle Receive.
// See Client.StartReceiver for details. This is a blocking call.
func (c *obsClient) StartReceiver(ctx context.Context, fn interface{}) error {
	ctx, r := observability.NewReporter(ctx, reportStartReceiver)

	err := c.client.StartReceiver(ctx, fn)

	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}
