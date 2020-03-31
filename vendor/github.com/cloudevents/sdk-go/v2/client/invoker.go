package client

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
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
		if err2 := m.Finish(err); err2 == nil {
			err = err2
		}
	}()

	e, err := binding.ToEvent(ctx, m)
	if err != nil {
		return err
	}

	if e != nil && r.fn != nil {
		resp, result := r.fn.invoke(ctx, *e)

		// Apply the defaulter chain to the outgoing event.
		if resp != nil && len(r.eventDefaulterFns) > 0 {
			for _, fn := range r.eventDefaulterFns {
				*resp = fn(ctx, *resp)
			}
			// Validate the event conforms to the CloudEvents Spec.
			if verr := resp.Validate(); verr != nil {
				cecontext.LoggerFrom(ctx).Error(fmt.Errorf("cloudevent validation failed on response event: %v, %w", verr, err))
			}
		}
		if respFn != nil {
			var rm binding.Message
			if resp != nil {
				rm = (*binding.EventMessage)(resp)
			}

			return respFn(ctx, rm, result) // TODO: there is a chance this never gets called. Is that ok?
		}
	}

	return nil
}

func (r *receiveInvoker) IsReceiver() bool {
	return !r.fn.hasEventOut
}

func (r *receiveInvoker) IsResponder() bool {
	return r.fn.hasEventOut
}
