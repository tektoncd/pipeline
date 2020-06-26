package http

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cloudevents/sdk-go/v2/binding"
	cecontext "github.com/cloudevents/sdk-go/v2/context"
	"github.com/cloudevents/sdk-go/v2/event"
	"github.com/cloudevents/sdk-go/v2/protocol"
)

const (
	// DefaultShutdownTimeout defines the default timeout given to the http.Server when calling Shutdown.
	DefaultShutdownTimeout = time.Minute * 1
)

type msgErr struct {
	msg    *Message
	respFn protocol.ResponseFn
	err    error
}

// Protocol acts as both a http client and a http handler.
type Protocol struct {
	Target          *url.URL
	RequestTemplate *http.Request
	Client          *http.Client
	incoming        chan msgErr

	// OptionsHandlerFn handles the OPTIONS method requests and is intended to
	// implement the abuse protection spec:
	// https://github.com/cloudevents/spec/blob/v1.0/http-webhook.md#4-abuse-protection
	OptionsHandlerFn http.HandlerFunc
	WebhookConfig    *WebhookConfig

	GetHandlerFn    http.HandlerFunc
	DeleteHandlerFn http.HandlerFunc

	// To support Opener:

	// ShutdownTimeout defines the timeout given to the http.Server when calling Shutdown.
	// If nil, DefaultShutdownTimeout is used.
	ShutdownTimeout time.Duration

	// Port is the port configured to bind the receiver to. Defaults to 8080.
	// If you want to know the effective port you're listening to, use GetListeningPort()
	Port int
	// Path is the path to bind the receiver to. Defaults to "/".
	Path string

	// Receive Mutex
	reMu sync.Mutex
	// Handler is the handler the http Server will use. Use this to reuse the
	// http server. If nil, the Protocol will create a one.
	Handler *http.ServeMux

	listener          atomic.Value
	roundTripper      http.RoundTripper
	server            *http.Server
	handlerRegistered bool
	middleware        []Middleware
}

func New(opts ...Option) (*Protocol, error) {
	p := &Protocol{
		incoming: make(chan msgErr),
		Port:     -1,
	}
	if err := p.applyOptions(opts...); err != nil {
		return nil, err
	}

	if p.Client == nil {
		p.Client = http.DefaultClient
	}

	if p.roundTripper != nil {
		p.Client.Transport = p.roundTripper
	}

	if p.ShutdownTimeout == 0 {
		p.ShutdownTimeout = DefaultShutdownTimeout
	}

	return p, nil
}

func (p *Protocol) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(p); err != nil {
			return err
		}
	}
	return nil
}

// Send implements binding.Sender
func (p *Protocol) Send(ctx context.Context, m binding.Message, transformers ...binding.Transformer) error {
	if ctx == nil {
		return fmt.Errorf("nil Context")
	} else if m == nil {
		return fmt.Errorf("nil Message")
	}

	_, err := p.Request(ctx, m, transformers...)
	return err
}

// Request implements binding.Requester
func (p *Protocol) Request(ctx context.Context, m binding.Message, transformers ...binding.Transformer) (binding.Message, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil Context")
	} else if m == nil {
		return nil, fmt.Errorf("nil Message")
	}

	var err error
	defer func() { _ = m.Finish(err) }()

	req := p.makeRequest(ctx)

	if p.Client == nil || req == nil || req.URL == nil {
		return nil, fmt.Errorf("not initialized: %#v", p)
	}

	if err = WriteRequest(ctx, m, req, transformers...); err != nil {
		return nil, err
	}

	return p.do(ctx, req)
}

func (p *Protocol) makeRequest(ctx context.Context) *http.Request {
	// TODO: support custom headers from context?
	req := &http.Request{
		Method: http.MethodPost,
		Header: make(http.Header),
		// TODO: HeaderFrom(ctx),
	}

	if p.RequestTemplate != nil {
		req.Method = p.RequestTemplate.Method
		req.URL = p.RequestTemplate.URL
		req.Close = p.RequestTemplate.Close
		req.Host = p.RequestTemplate.Host
		copyHeadersEnsure(p.RequestTemplate.Header, &req.Header)
	}

	if p.Target != nil {
		req.URL = p.Target
	}

	// Override the default request with target from context.
	if target := cecontext.TargetFrom(ctx); target != nil {
		req.URL = target
	}
	return req.WithContext(ctx)
}

// Ensure to is a non-nil map before copying
func copyHeadersEnsure(from http.Header, to *http.Header) {
	if len(from) > 0 {
		if *to == nil {
			*to = http.Header{}
		}
		copyHeaders(from, *to)
	}
}

func copyHeaders(from, to http.Header) {
	if from == nil || to == nil {
		return
	}
	for header, values := range from {
		for _, value := range values {
			to.Add(header, value)
		}
	}
}

// Receive the next incoming HTTP request as a CloudEvent.
// Returns non-nil error if the incoming HTTP request fails to parse as a CloudEvent
// Returns io.EOF if the receiver is closed.
func (p *Protocol) Receive(ctx context.Context) (binding.Message, error) {
	if ctx == nil {
		return nil, fmt.Errorf("nil Context")
	}

	msg, fn, err := p.Respond(ctx)
	// No-op the response when finish is invoked.
	if msg != nil {
		return binding.WithFinish(msg, func(err error) {
			if fn != nil {
				_ = fn(ctx, nil, nil)
			}
		}), err
	} else {
		return nil, err
	}
}

// Respond receives the next incoming HTTP request as a CloudEvent and waits
// for the response callback to invoked before continuing.
// Returns non-nil error if the incoming HTTP request fails to parse as a CloudEvent
// Returns io.EOF if the receiver is closed.
func (p *Protocol) Respond(ctx context.Context) (binding.Message, protocol.ResponseFn, error) {
	if ctx == nil {
		return nil, nil, fmt.Errorf("nil Context")
	}

	select {
	case in, ok := <-p.incoming:
		if !ok {
			return nil, nil, io.EOF
		}

		if in.msg == nil {
			return nil, in.respFn, in.err
		}
		return in.msg, in.respFn, in.err

	case <-ctx.Done():
		return nil, nil, io.EOF
	}
}

// ServeHTTP implements http.Handler.
// Blocks until ResponseFn is invoked.
func (p *Protocol) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	// Filter the GET style methods:
	switch req.Method {
	case http.MethodOptions:
		if p.OptionsHandlerFn == nil {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		p.OptionsHandlerFn(rw, req)
		return

	case http.MethodGet:
		if p.GetHandlerFn == nil {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		p.GetHandlerFn(rw, req)
		return

	case http.MethodDelete:
		if p.DeleteHandlerFn == nil {
			rw.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		p.DeleteHandlerFn(rw, req)
		return
	}

	m := NewMessageFromHttpRequest(req)
	if m == nil {
		// Should never get here unless ServeHTTP is called directly.
		p.incoming <- msgErr{msg: nil, err: binding.ErrUnknownEncoding}
		rw.WriteHeader(http.StatusBadRequest)
		return // if there was no message, return.
	}

	var finishErr error
	m.OnFinish = func(err error) error {
		finishErr = err
		return nil
	}

	wg := sync.WaitGroup{}
	wg.Add(1)
	var fn protocol.ResponseFn = func(ctx context.Context, respMsg binding.Message, res protocol.Result, transformers ...binding.Transformer) error {
		// Unblock the ServeHTTP after the reply is written
		defer func() {
			wg.Done()
		}()

		if finishErr != nil {
			http.Error(rw, fmt.Sprintf("Cannot forward CloudEvent: %s", finishErr), http.StatusInternalServerError)
			return finishErr
		}

		status := http.StatusOK
		if res != nil {
			var result *Result
			switch {
			case protocol.ResultAs(res, &result):
				if result.StatusCode > 100 && result.StatusCode < 600 {
					status = result.StatusCode
				}

			case !protocol.IsACK(res):
				// Map client errors to http status code
				validationError := event.ValidationError{}
				if errors.As(res, &validationError) {
					status = http.StatusBadRequest
					rw.Header().Set("content-type", "text/plain")
					rw.WriteHeader(status)
					_, _ = rw.Write([]byte(validationError.Error()))
					return validationError
				} else if errors.Is(res, binding.ErrUnknownEncoding) {
					status = http.StatusUnsupportedMediaType
				} else {
					status = http.StatusInternalServerError
				}
			}
		}

		if respMsg != nil {
			err := WriteResponseWriter(ctx, respMsg, status, rw, transformers...)
			return respMsg.Finish(err)
		}

		rw.WriteHeader(status)
		return nil
	}

	p.incoming <- msgErr{msg: m, respFn: fn} // Send to Request
	// Block until ResponseFn is invoked
	wg.Wait()
}
