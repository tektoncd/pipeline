package http

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
)

// Transport adheres to transport.Transport.
var _ transport.Transport = (*Transport)(nil)

const (
	// DefaultShutdownTimeout defines the default timeout given to the http.Server when calling Shutdown.
	DefaultShutdownTimeout = time.Minute * 1

	// TransportName is the name of this transport.
	TransportName = "HTTP"
)

// Transport acts as both a http client and a http handler.
type Transport struct {
	// The encoding used to select the codec for outbound events.
	Encoding Encoding

	// DefaultEncodingSelectionFn allows for other encoding selection strategies to be injected.
	DefaultEncodingSelectionFn EncodingSelector

	// ShutdownTimeout defines the timeout given to the http.Server when calling Shutdown.
	// If nil, DefaultShutdownTimeout is used.
	ShutdownTimeout *time.Duration

	// Sending

	// Client is the http client that will be used to send requests.
	// If nil, the Transport will create a one.
	Client *http.Client
	// Req is the base http request that is used for http.Do.
	// Only .Method, .URL, .Close, and .Header is considered.
	// If not set, Req.Method defaults to POST.
	// Req.URL or context.WithTarget(url) are required for sending.
	Req *http.Request

	// Receiving

	// Receiver is invoked target for incoming events.
	Receiver transport.Receiver
	// Converter is invoked if the incoming transport receives an undecodable
	// message.
	Converter transport.Converter
	// Port is the port to bind the receiver to. Defaults to 8080.
	Port *int
	// Path is the path to bind the receiver to. Defaults to "/".
	Path string
	// Handler is the handler the http Server will use. Use this to reuse the
	// http server. If nil, the Transport will create a one.
	Handler *http.ServeMux

	// LongPollClient is the http client that will be used to long poll.
	// If nil and LongPollReq is set, the Transport will create a one.
	LongPollClient *http.Client
	// LongPollReq is the base http request that is used for long poll.
	// Only .Method, .URL, .Close, and .Header is considered.
	// If not set, LongPollReq.Method defaults to GET.
	// LongPollReq.URL or context.WithLongPollTarget(url) are required to long
	// poll on StartReceiver.
	LongPollReq *http.Request

	listener          net.Listener
	server            *http.Server
	handlerRegistered bool
	codec             transport.Codec
	// Create Mutex
	crMu sync.Mutex
	// Receive Mutex
	reMu sync.Mutex

	middleware []Middleware
}

func New(opts ...Option) (*Transport, error) {
	t := &Transport{
		Req: &http.Request{
			Method: http.MethodPost,
		},
	}
	if err := t.applyOptions(opts...); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *Transport) applyOptions(opts ...Option) error {
	for _, fn := range opts {
		if err := fn(t); err != nil {
			return err
		}
	}
	return nil
}

func (t *Transport) loadCodec(ctx context.Context) bool {
	if t.codec == nil {
		t.crMu.Lock()
		if t.DefaultEncodingSelectionFn != nil && t.Encoding != Default {
			logger := cecontext.LoggerFrom(ctx)
			logger.Warn("transport has a DefaultEncodingSelectionFn set but Encoding is not Default. DefaultEncodingSelectionFn will be ignored.")

			t.codec = &Codec{
				Encoding: t.Encoding,
			}
		} else {
			t.codec = &Codec{
				Encoding:                   t.Encoding,
				DefaultEncodingSelectionFn: t.DefaultEncodingSelectionFn,
			}
		}
		t.crMu.Unlock()
	}
	return true
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

// Ensure to is a non-nil map before copying
func copyHeadersEnsure(from http.Header, to *http.Header) {
	if len(from) > 0 {
		if *to == nil {
			*to = http.Header{}
		}
		copyHeaders(from, *to)
	}
}

// Send implements Transport.Send
func (t *Transport) Send(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	ctx, r := observability.NewReporter(ctx, reportSend)
	rctx, resp, err := t.obsSend(ctx, event)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return rctx, resp, err
}

func (t *Transport) obsSend(ctx context.Context, event cloudevents.Event) (context.Context, *cloudevents.Event, error) {
	if t.Client == nil {
		t.crMu.Lock()
		if t.Client == nil {
			t.Client = &http.Client{}
		}
		t.crMu.Unlock()
	}

	req := http.Request{
		Header: HeaderFrom(ctx),
	}
	if t.Req != nil {
		req.Method = t.Req.Method
		req.URL = t.Req.URL
		req.Close = t.Req.Close
		req.Host = t.Req.Host
		copyHeadersEnsure(t.Req.Header, &req.Header)
	}

	// Override the default request with target from context.
	if target := cecontext.TargetFrom(ctx); target != nil {
		req.URL = target
	}

	if ok := t.loadCodec(ctx); !ok {
		return WithTransportContext(ctx, NewTransportContextFromResponse(nil)), nil, fmt.Errorf("unknown encoding set on transport: %d", t.Encoding)
	}

	msg, err := t.codec.Encode(ctx, event)
	if err != nil {
		return WithTransportContext(ctx, NewTransportContextFromResponse(nil)), nil, err
	}

	if m, ok := msg.(*Message); ok {
		m.ToRequest(&req)
		return httpDo(ctx, t.Client, &req, func(resp *http.Response, err error) (context.Context, *cloudevents.Event, error) {
			rctx := WithTransportContext(ctx, NewTransportContextFromResponse(resp))
			if err != nil {
				return rctx, nil, err
			}
			defer resp.Body.Close()

			body, _ := ioutil.ReadAll(resp.Body)
			respEvent, err := t.MessageToEvent(ctx, &Message{
				Header: resp.Header,
				Body:   body,
			})
			if err != nil {
				isErr := true
				handled := false
				if txerr, ok := err.(*transport.ErrTransportMessageConversion); ok {
					if !txerr.IsFatal() {
						isErr = false
					}
					if txerr.Handled() {
						handled = true
					}
				}
				if isErr {
					return rctx, nil, err
				}
				if handled {
					return rctx, nil, nil
				}
			}
			if accepted(resp) {
				return rctx, respEvent, nil
			}
			return rctx, respEvent, fmt.Errorf("error sending cloudevent: %s", resp.Status)
		})
	}
	return WithTransportContext(ctx, NewTransportContextFromResponse(nil)), nil, fmt.Errorf("failed to encode Event into a Message")
}

func (t *Transport) MessageToEvent(ctx context.Context, msg *Message) (*cloudevents.Event, error) {
	logger := cecontext.LoggerFrom(ctx)
	var event *cloudevents.Event
	var err error

	if msg.CloudEventsVersion() != "" {
		// This is likely a cloudevents encoded message, try to decode it.
		if ok := t.loadCodec(ctx); !ok {
			err = transport.NewErrTransportMessageConversion("http", fmt.Sprintf("unknown encoding set on transport: %d", t.Encoding), false, true)
			logger.Error("failed to load codec", zap.Error(err))
		} else {
			event, err = t.codec.Decode(ctx, msg)
		}
	} else {
		err = transport.NewErrTransportMessageConversion("http", "cloudevents version unknown", false, false)
	}

	// If codec returns and error, or could not load the correct codec, try
	// with the converter if it is set.
	if err != nil && t.HasConverter() {
		event, err = t.Converter.Convert(ctx, msg, err)
	}

	// If err is still set, it means that there was no converter, or the
	// converter failed to convert.
	if err != nil {
		logger.Debug("failed to decode message", zap.Error(err))
	}

	// If event and error are both nil, then there is nothing to do with this event, it was handled.
	if err == nil && event == nil {
		logger.Debug("convert function returned (nil, nil)")
		err = transport.NewErrTransportMessageConversion("http", "convert function handled request", true, false)
	}

	return event, err
}

// SetReceiver implements Transport.SetReceiver
func (t *Transport) SetReceiver(r transport.Receiver) {
	t.Receiver = r
}

// SetConverter implements Transport.SetConverter
func (t *Transport) SetConverter(c transport.Converter) {
	t.Converter = c
}

// HasConverter implements Transport.HasConverter
func (t *Transport) HasConverter() bool {
	return t.Converter != nil
}

// StartReceiver implements Transport.StartReceiver
// NOTE: This is a blocking call.
func (t *Transport) StartReceiver(ctx context.Context) error {
	t.reMu.Lock()
	defer t.reMu.Unlock()

	if t.LongPollReq != nil {
		go func() { _ = t.longPollStart(ctx) }()
	}

	if t.Handler == nil {
		t.Handler = http.NewServeMux()
	}
	if !t.handlerRegistered {
		// handler.Handle might panic if the user tries to use the same path as the sdk.
		t.Handler.Handle(t.GetPath(), t)
		t.handlerRegistered = true
	}

	addr, err := t.listen()
	if err != nil {
		return err
	}

	t.server = &http.Server{
		Addr:    addr.String(),
		Handler: attachMiddleware(t.Handler, t.middleware),
	}

	// Shutdown
	defer func() {
		t.server.Close()
		t.server = nil
	}()

	errChan := make(chan error, 1)
	go func() {
		errChan <- t.server.Serve(t.listener)
	}()

	// wait for the server to return or ctx.Done().
	select {
	case <-ctx.Done():
		// Try a gracefully shutdown.
		timeout := DefaultShutdownTimeout
		if t.ShutdownTimeout != nil {
			timeout = *t.ShutdownTimeout
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := t.server.Shutdown(ctx)
		<-errChan // Wait for server goroutine to exit
		return err
	case err := <-errChan:
		return err
	}
}

func (t *Transport) longPollStart(ctx context.Context) error {
	logger := cecontext.LoggerFrom(ctx)
	logger.Info("starting long poll receiver")

	if t.LongPollClient == nil {
		t.crMu.Lock()
		t.LongPollClient = &http.Client{}
		t.crMu.Unlock()
	}
	req := &http.Request{
		// TODO: decide if it is ok to use HeaderFrom context here.
		Header: HeaderFrom(ctx),
	}
	if t.LongPollReq != nil {
		req.Method = t.LongPollReq.Method
		req.URL = t.LongPollReq.URL
		req.Close = t.LongPollReq.Close
		copyHeaders(t.LongPollReq.Header, req.Header)
	}

	// Override the default request with target from context.
	if target := LongPollTargetFrom(ctx); target != nil {
		req.URL = target
	}

	if req.URL == nil {
		return errors.New("no long poll target found")
	}

	req = req.WithContext(ctx)
	msgCh := make(chan Message)
	defer close(msgCh)
	isClosed := false

	go func(ch chan<- Message) {
		for {
			if isClosed {
				return
			}

			if resp, err := t.LongPollClient.Do(req); err != nil {
				logger.Errorw("long poll request returned error", err)
				uErr := err.(*url.Error)
				if uErr.Temporary() || uErr.Timeout() {
					continue
				}
				// TODO: if the transport is throwing errors, we might want to try again. Maybe with a back-off sleep.
				// But this error also might be that there was a done on the context.
			} else if resp.StatusCode == http.StatusNotModified {
				// Keep polling.
				continue
			} else if resp.StatusCode == http.StatusOK {
				body, _ := ioutil.ReadAll(resp.Body)
				if err := resp.Body.Close(); err != nil {
					logger.Warnw("error closing long poll response body", zap.Error(err))
				}
				msg := Message{
					Header: resp.Header,
					Body:   body,
				}
				msgCh <- msg
			} else {
				// TODO: not sure what to do with upstream errors yet.
				logger.Errorw("unhandled long poll response", zap.Any("resp", resp))
			}
		}
	}(msgCh)

	// Attach the long poll request context to the context.
	ctx = WithTransportContext(ctx, TransportContext{
		URI:    req.URL.RequestURI(),
		Host:   req.URL.Host,
		Method: req.Method,
	})

	for {
		select {
		case <-ctx.Done():
			isClosed = true
			return nil
		case msg := <-msgCh:
			logger.Debug("got a message", zap.Any("msg", msg))
			if event, err := t.MessageToEvent(ctx, &msg); err != nil {
				logger.Errorw("could not convert http message to event", zap.Error(err))
			} else {
				logger.Debugw("got an event", zap.Any("event", event))
				// TODO: deliver event.
				if _, err := t.invokeReceiver(ctx, *event); err != nil {
					logger.Errorw("could not invoke receiver event", zap.Error(err))
				}
			}
		}
	}
}

// attachMiddleware attaches the HTTP middleware to the specified handler.
func attachMiddleware(h http.Handler, middleware []Middleware) http.Handler {
	for _, m := range middleware {
		h = m(h)
	}
	return h
}

type eventError struct {
	ctx   context.Context
	event *cloudevents.Event
	err   error
}

func httpDo(ctx context.Context, client *http.Client, req *http.Request, fn func(*http.Response, error) (context.Context, *cloudevents.Event, error)) (context.Context, *cloudevents.Event, error) {
	// Run the HTTP request in a goroutine and pass the response to fn.
	c := make(chan eventError, 1)
	req = req.WithContext(ctx)
	go func() {
		rctx, event, err := fn(client.Do(req))
		c <- eventError{ctx: rctx, event: event, err: err}
	}()
	select {
	case <-ctx.Done():
		return ctx, nil, ctx.Err()
	case ee := <-c:
		return ee.ctx, ee.event, ee.err
	}
}

// accepted is a helper method to understand if the response from the target
// accepted the CloudEvent.
func accepted(resp *http.Response) bool {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}
	return false
}

func (t *Transport) invokeReceiver(ctx context.Context, event cloudevents.Event) (*Response, error) {
	ctx, r := observability.NewReporter(ctx, reportReceive)
	resp, err := t.obsInvokeReceiver(ctx, event)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return resp, err
}

func (t *Transport) obsInvokeReceiver(ctx context.Context, event cloudevents.Event) (*Response, error) {
	logger := cecontext.LoggerFrom(ctx)
	if t.Receiver != nil {
		// Note: http does not use eventResp.Reason
		eventResp := cloudevents.EventResponse{}
		resp := Response{}

		err := t.Receiver.Receive(ctx, event, &eventResp)
		if err != nil {
			logger.Warnw("got an error from receiver fn", zap.Error(err))
			resp.StatusCode = http.StatusInternalServerError
			return &resp, err
		}

		if eventResp.Event != nil {
			if t.loadCodec(ctx) {
				if m, err := t.codec.Encode(ctx, *eventResp.Event); err != nil {
					logger.Errorw("failed to encode response from receiver fn", zap.Error(err))
				} else if msg, ok := m.(*Message); ok {
					resp.Message = *msg
				}
			} else {
				logger.Error("failed to load codec")
				resp.StatusCode = http.StatusInternalServerError
				return &resp, err
			}
			// Look for a transport response context
			var trx *TransportResponseContext
			if ptrTrx, ok := eventResp.Context.(*TransportResponseContext); ok {
				// found a *TransportResponseContext, use it.
				trx = ptrTrx
			} else if realTrx, ok := eventResp.Context.(TransportResponseContext); ok {
				// found a TransportResponseContext, make it a pointer.
				trx = &realTrx
			}
			// If we found a TransportResponseContext, use it.
			if trx != nil && trx.Header != nil && len(trx.Header) > 0 {
				copyHeadersEnsure(trx.Header, &resp.Message.Header)
			}
		}

		if eventResp.Status != 0 {
			resp.StatusCode = eventResp.Status
		} else {
			resp.StatusCode = http.StatusAccepted // default is 202 - Accepted
		}
		return &resp, err
	}
	return nil, nil
}

// ServeHTTP implements http.Handler
func (t *Transport) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	ctx, r := observability.NewReporter(req.Context(), reportServeHTTP)
	// Add the transport context to ctx.
	ctx = WithTransportContext(ctx, NewTransportContext(req))
	logger := cecontext.LoggerFrom(ctx)

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logger.Errorw("failed to handle request", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Invalid request"}`))
		r.Error()
		return
	}

	event, err := t.MessageToEvent(ctx, &Message{
		Header: req.Header,
		Body:   body,
	})
	if err != nil {
		isFatal := true
		handled := false
		if txerr, ok := err.(*transport.ErrTransportMessageConversion); ok {
			isFatal = txerr.IsFatal()
			handled = txerr.Handled()
		}
		if isFatal {
			logger.Errorw("failed to convert http message to event", zap.Error(err))
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
			r.Error()
			return
		}
		// if handled, do not pass to receiver.
		if handled {
			w.WriteHeader(http.StatusNoContent)
			r.OK()
			return
		}
	}
	if event == nil {
		logger.Error("failed to get non-nil event from MessageToEvent")
		w.WriteHeader(http.StatusBadRequest)
		r.Error()
		return
	}

	resp, err := t.invokeReceiver(ctx, *event)
	if err != nil {
		logger.Warnw("error returned from invokeReceiver", zap.Error(err))
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(fmt.Sprintf(`{"error":%q}`, err.Error())))
		r.Error()
		return
	}

	if resp != nil {
		if t.Req != nil {
			copyHeaders(t.Req.Header, w.Header())
		}
		if len(resp.Message.Header) > 0 {
			copyHeaders(resp.Message.Header, w.Header())
		}

		status := http.StatusAccepted
		if resp.StatusCode >= 200 && resp.StatusCode < 600 {
			status = resp.StatusCode
		}
		w.Header().Add("Content-Length", strconv.Itoa(len(resp.Message.Body)))
		w.WriteHeader(status)

		if len(resp.Message.Body) > 0 {
			if _, err := w.Write(resp.Message.Body); err != nil {
				r.Error()
				return
			}
		}

		r.OK()
		return
	}

	w.WriteHeader(http.StatusNoContent)
	r.OK()
}

// GetPort returns the listening port.
// Returns -1 if there is a listening error.
// Note this will call net.Listen() if  the listener is not already started.
func (t *Transport) GetPort() int {
	// Ensure we have a listener and therefore a port.
	if _, err := t.listen(); err == nil || t.Port != nil {
		return *t.Port
	}
	return -1
}

func (t *Transport) setPort(port int) {
	if t.Port == nil {
		t.Port = new(int)
	}
	*t.Port = port
}

// listen if not already listening, update t.Port
func (t *Transport) listen() (net.Addr, error) {
	if t.listener == nil {
		port := 8080
		if t.Port != nil {
			port = *t.Port
			if port < 0 || port > 65535 {
				return nil, fmt.Errorf("invalid port %d", port)
			}
		}
		var err error
		if t.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port)); err != nil {
			return nil, err
		}
	}
	addr := t.listener.Addr()
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		t.setPort(tcpAddr.Port)
	}
	return addr, nil
}

// GetPath returns the path the transport is hosted on. If the path is '/',
// the transport will handle requests on any URI. To discover the true path
// a request was received on, inspect the context from Receive(cxt, ...) with
// TransportContextFrom(ctx).
func (t *Transport) GetPath() string {
	path := strings.TrimSpace(t.Path)
	if len(path) > 0 {
		return path
	}
	return "/" // default
}
