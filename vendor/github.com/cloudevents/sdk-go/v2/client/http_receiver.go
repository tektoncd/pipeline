package client

import (
	"context"
	"net/http"
	"sync"

	thttp "github.com/cloudevents/sdk-go/v2/protocol/http"
)

func NewHTTPReceiveHandler(ctx context.Context, p *thttp.Protocol, fn interface{}) (*EventReceiver, error) {
	invoker, err := newReceiveInvoker(fn)
	if err != nil {
		return nil, err
	}

	return &EventReceiver{
		p:       p,
		invoker: invoker,
	}, nil
}

type EventReceiver struct {
	p       *thttp.Protocol
	invoker Invoker
}

func (r *EventReceiver) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		r.p.ServeHTTP(rw, req)
		wg.Done()
	}()

	ctx := req.Context()
	msg, respFn, err := r.p.Respond(ctx)
	if err != nil {
		//lint:ignore SA9003 TODO: Branch left empty
	} else if err := r.invoker.Invoke(ctx, msg, respFn); err != nil {
		// TODO
	}
	// Block until ServeHTTP has returned
	wg.Wait()
}
