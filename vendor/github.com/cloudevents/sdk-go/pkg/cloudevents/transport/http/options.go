package http

import (
	"fmt"
	nethttp "net/http"
	"net/url"
	"strings"
	"time"
)

// Option is the function signature required to be considered an http.Option.
type Option func(*Transport) error

// WithTarget sets the outbound recipient of cloudevents when using an HTTP
// request.
func WithTarget(targetUrl string) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http target option can not set nil transport")
		}
		targetUrl = strings.TrimSpace(targetUrl)
		if targetUrl != "" {
			var err error
			var target *url.URL
			target, err = url.Parse(targetUrl)
			if err != nil {
				return fmt.Errorf("http target option failed to parse target url: %s", err.Error())
			}

			if t.Req == nil {
				t.Req = &nethttp.Request{
					Method: nethttp.MethodPost,
				}
			}
			t.Req.URL = target
			return nil
		}
		return fmt.Errorf("http target option was empty string")
	}
}

// WithMethod sets the outbound recipient of cloudevents when using an HTTP
// request.
func WithMethod(method string) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http method option can not set nil transport")
		}
		method = strings.TrimSpace(method)
		if method != "" {
			if t.Req == nil {
				t.Req = &nethttp.Request{}
			}
			t.Req.Method = method
			return nil
		}
		return fmt.Errorf("http method option was empty string")
	}
}

// WithHeader sets an additional default outbound header for all cloudevents
// when using an HTTP request.
func WithHeader(key, value string) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http header option can not set nil transport")
		}
		key = strings.TrimSpace(key)
		if key != "" {
			if t.Req == nil {
				t.Req = &nethttp.Request{}
			}
			if t.Req.Header == nil {
				t.Req.Header = nethttp.Header{}
			}
			t.Req.Header.Add(key, value)
			return nil
		}
		return fmt.Errorf("http header option was empty string")
	}
}

// WithShutdownTimeout sets the shutdown timeout when the http server is being shutdown.
func WithShutdownTimeout(timeout time.Duration) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http shutdown timeout option can not set nil transport")
		}
		t.ShutdownTimeout = &timeout
		return nil
	}
}

// WithEncoding sets the encoding for clients with HTTP transports.
func WithEncoding(encoding Encoding) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http encoding option can not set nil transport")
		}
		t.Encoding = encoding
		return nil
	}
}

// WithDefaultEncodingSelector sets the encoding selection strategy for
// default encoding selections based on Event.
func WithDefaultEncodingSelector(fn EncodingSelector) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http default encoding selector option can not set nil transport")
		}
		if fn != nil {
			t.DefaultEncodingSelectionFn = fn
			return nil
		}
		return fmt.Errorf("http fn for DefaultEncodingSelector was nil")
	}
}

// WithBinaryEncoding sets the encoding selection strategy for
// default encoding selections based on Event, the encoded event will be the
// given version in Binary form.
func WithBinaryEncoding() Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http binary encoding option can not set nil transport")
		}

		t.DefaultEncodingSelectionFn = DefaultBinaryEncodingSelectionStrategy
		return nil
	}
}

// WithStructuredEncoding sets the encoding selection strategy for
// default encoding selections based on Event, the encoded event will be the
// given version in Structured form.
func WithStructuredEncoding() Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http structured encoding option can not set nil transport")
		}

		t.DefaultEncodingSelectionFn = DefaultStructuredEncodingSelectionStrategy
		return nil
	}
}

// WithPort sets the port for for clients with HTTP transports.
func WithPort(port int) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http port option can not set nil transport")
		}
		if port < 0 {
			return fmt.Errorf("http port option was given an invalid port: %d", port)
		}
		t.Port = &port
		return nil
	}
}

// WithPath sets the path to receive cloudevents on for HTTP transports.
func WithPath(path string) Option {
	return func(t *Transport) error {
		if t == nil {
			return fmt.Errorf("http path option can not set nil transport")
		}
		path = strings.TrimSpace(path)
		if len(path) == 0 {
			return fmt.Errorf("http path option was given an invalid path: %q", path)
		}
		t.Path = path
		return nil
	}
}

// Middleware is a function that takes an existing http.Handler and wraps it in middleware,
// returning the wrapped http.Handler.
type Middleware func(next nethttp.Handler) nethttp.Handler

// WithMiddleware adds an HTTP middleware to the transport. It may be specified multiple times.
// Middleware is applied to everything before it. For example
// `NewClient(WithMiddleware(foo), WithMiddleware(bar))` would result in `bar(foo(original))`.
func WithMiddleware(middleware Middleware) Option {
	return func (t *Transport) error {
		if t == nil {
			return fmt.Errorf("http middleware option can not set nil transport")
		}
		t.middleware = append(t.middleware, middleware)
		return nil
	}
}
