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

func (p *Protocol) OpenInbound(ctx context.Context) error {
	p.reMu.Lock()
	defer p.reMu.Unlock()

	if p.Handler == nil {
		p.Handler = http.NewServeMux()
	}

	if !p.handlerRegistered {
		// handler.Handle might panic if the user tries to use the same path as the sdk.
		p.Handler.Handle(p.GetPath(), p)
		p.handlerRegistered = true
	}

	addr, err := p.listen()
	if err != nil {
		return err
	}

	p.server = &http.Server{
		Addr: addr.String(),
		Handler: &ochttp.Handler{
			Propagation:    &tracecontext.HTTPFormat{},
			Handler:        attachMiddleware(p.Handler, p.middleware),
			FormatSpanName: formatSpanName,
		},
	}

	// Shutdown
	defer func() {
		_ = p.server.Close()
		p.server = nil
	}()

	errChan := make(chan error, 1)
	go func() {
		errChan <- p.server.Serve(p.listener)
	}()

	// nil check and default
	shutdown := DefaultShutdownTimeout
	if p.ShutdownTimeout != nil {
		shutdown = *p.ShutdownTimeout
	}

	// wait for the server to return or ctx.Done().
	select {
	case <-ctx.Done():
		// Try a gracefully shutdown.
		timeout := shutdown
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		err := p.server.Shutdown(ctx)
		<-errChan // Wait for server goroutine to exit
		return err
	case err := <-errChan:
		return err
	}
}

// GetPort returns the listening port.
// Returns -1 if there is a listening error.
// Note this will call net.Listen() if  the listener is not already started.
func (p *Protocol) GetPort() int {
	// Ensure we have a listener and therefore a port.
	if _, err := p.listen(); err == nil || p.Port != nil {
		return *p.Port
	}
	return -1
}

func formatSpanName(r *http.Request) string {
	return "cloudevents.http." + r.URL.Path
}

func (p *Protocol) setPort(port int) {
	if p.Port == nil {
		p.Port = new(int)
	}
	*p.Port = port
}

// listen if not already listening, update t.Port
func (p *Protocol) listen() (net.Addr, error) {
	if p.listener == nil {
		port := 8080
		if p.Port != nil {
			port = *p.Port
			if port < 0 || port > 65535 {
				return nil, fmt.Errorf("invalid port %d", port)
			}
		}
		var err error
		if p.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", port)); err != nil {
			return nil, err
		}
	}
	addr := p.listener.Addr()
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		p.setPort(tcpAddr.Port)
	}
	return addr, nil
}

// GetPath returns the path the transport is hosted on. If the path is '/',
// the transport will handle requests on any URI. To discover the true path
// a request was received on, inspect the context from Receive(cxt, ...) with
// TransportContextFrom(ctx).
func (p *Protocol) GetPath() string {
	path := strings.TrimSpace(p.Path)
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
