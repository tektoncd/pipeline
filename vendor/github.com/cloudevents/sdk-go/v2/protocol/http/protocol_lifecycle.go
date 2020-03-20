package http

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/cloudevents/sdk-go/v2/protocol"
	"go.opencensus.io/plugin/ochttp"
	"go.opencensus.io/plugin/ochttp/propagation/tracecontext"
)

var _ protocol.Opener = (*Protocol)(nil)

func (e *Protocol) OpenInbound(ctx context.Context) error {
	e.reMu.Lock()
	defer e.reMu.Unlock()

	if e.Handler == nil {
		e.Handler = http.NewServeMux()
	}

	if !e.handlerRegistered {
		// handler.Handle might panic if the user tries to use the same path as the sdk.
		e.Handler.Handle(e.GetPath(), e)
		e.handlerRegistered = true
	}

	addr, err := e.listen()
	if err != nil {
		return err
	}

	e.server = &http.Server{
		Addr: addr.String(),
		Handler: &ochttp.Handler{
			Propagation:    &tracecontext.HTTPFormat{},
			Handler:        attachMiddleware(e.Handler, e.middleware),
			FormatSpanName: formatSpanName,
		},
	}

	// Shutdown
	defer func() {
		_ = e.server.Close()
		e.server = nil
	}()

	errChan := make(chan error, 1)
	go func() {
		errChan <- e.server.Serve(e.listener)
	}()

	// nil check and default
	shutdown := DefaultShutdownTimeout
	if e.ShutdownTimeout != nil {
		shutdown = *e.ShutdownTimeout
	}

	// wait for the server to return or ctx.Done().
	select {
	case <-ctx.Done():
		// Try a gracefully shutdown.
		timeout := shutdown
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := e.server.Shutdown(ctx)
		<-errChan // Wait for server goroutine to exit
		return err
	case err := <-errChan:
		return err
	}
}

// GetPort returns the listening port.
// Returns -1 if there is a listening error.
// Note this will call net.Listen() if  the listener is not already started.
func (e *Protocol) GetPort() int {
	// Ensure we have a listener and therefore a port.
	if _, err := e.listen(); err == nil || e.Port != nil {
		return *e.Port
	}
	return -1
}

func formatSpanName(r *http.Request) string {
	return "cloudevents.http." + r.URL.Path
}

func (e *Protocol) setPort(port int) {
	if e.Port == nil {
		e.Port = new(int)
	}
	*e.Port = port
}

// listen if not already listening, update t.Port
func (e *Protocol) listen() (net.Addr, error) {
	if e.listener == nil {
		port := 8080
		if e.Port != nil {
			port = *e.Port
			if port < 0 || port > 65535 {
				return nil, fmt.Errorf("invalid port %d", port)
			}
		}
		var err error
		if e.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port)); err != nil {
			return nil, err
		}
	}
	addr := e.listener.Addr()
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		e.setPort(tcpAddr.Port)
	}
	return addr, nil
}

// GetPath returns the path the transport is hosted on. If the path is '/',
// the transport will handle requests on any URI. To discover the true path
// a request was received on, inspect the context from Receive(cxt, ...) with
// TransportContextFrom(ctx).
func (e *Protocol) GetPath() string {
	path := strings.TrimSpace(e.Path)
	if len(path) > 0 {
		return path
	}
	return "/" // default
}

// attachMiddleware attaches the HTTP middleware to the specified handler.
func attachMiddleware(h http.Handler, middleware []Middleware) http.Handler {
	for _, m := range middleware {
		h = m(h)
	}
	return h
}
