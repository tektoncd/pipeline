package client

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

type Invoker interface {
	Invoke(context.Context, binding.Message, protocol.ResponseFn) error
	IsReceiver() bool
	IsResponder() bool
}

var _ Invoker = (*receiveInvoker)(nil)

func newReceiveInvoker(fn interface{}, fns ...EventDefaulter) (Invoker, error) {
	r := &receiveInvoker{
		eventDefaulterFns: fns,
	}

	if fn, err := receiver(fn); err != nil {
		return nil, err
	} else {
		r.fn = fn
	}

	return r, nil
}

type receiveInvoker struct {
	fn                *receiverFn
	eventDefaulterFns []EventDefaulter
}

func (r *receiveInvoker) Invoke(ctx context.Context, m binding.Message, respFn protocol.ResponseFn) (err error) {
	defer func() {
		err = m.Finish(err)
	}()

	var respMsg binding.Message
	var result protocol.Result

	e, eventErr := binding.ToEvent(ctx, m)
	switch {
	case eventErr != nil && r.fn.hasEventIn:
		return respFn(ctx, nil, protocol.NewReceipt(false, "failed to convert Message to Event: %w", eventErr))
	case r.fn != nil:
		// Check if event is valid before invoking the receiver function
		if e != nil {
			if validationErr := e.Validate(); validationErr != nil {
				return respFn(ctx, nil, protocol.NewReceipt(false, "validation error in incoming event: %w", validationErr))
			}
		}

		// Let's invoke the receiver fn
		var resp *event.Event
		resp, result = r.fn.invoke(ctx, e)

		if respFn == nil {
			break
		}

		// Apply the defaulter chain to the outgoing event.
		if resp != nil && len(r.eventDefaulterFns) > 0 {
			for _, fn := range r.eventDefaulterFns {
				*resp = fn(ctx, *resp)
			}
			// Validate the event conforms to the CloudEvents Spec.
			if vErr := resp.Validate(); vErr != nil {
				cecontext.LoggerFrom(ctx).Errorf("cloudevent validation failed on response event: %w", vErr)
			}
		}

		// because binding.Message is an interface, casting a nil resp
		// here would make future comparisons to nil false
		if resp != nil {
			respMsg = (*binding.EventMessage)(resp)
		}
	}

	if respFn == nil {
		// let the protocol ACK based on the result
		return result
	}

	return respFn(ctx, respMsg, result)
}

func (r *receiveInvoker) IsReceiver() bool {
	return !r.fn.hasEventOut
}

func (r *receiveInvoker) IsResponder() bool {
	return r.fn.hasEventOut
}
