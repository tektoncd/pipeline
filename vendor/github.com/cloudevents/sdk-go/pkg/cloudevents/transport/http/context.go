package http

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// TransportContext allows a Receiver to understand the context of a request.
type TransportContext struct {
	URI        string
	Host       string
	Method     string
	Header     http.Header
	StatusCode int

	// IgnoreHeaderPrefixes controls what comes back from AttendToHeaders.
	// AttendToHeaders controls what is output for .String()
	IgnoreHeaderPrefixes []string
}

// NewTransportContext creates a new TransportContext from a http.Request.
func NewTransportContext(req *http.Request) TransportContext {
	var tx *TransportContext
	if req != nil {
		tx = &TransportContext{
			URI:    req.RequestURI,
			Host:   req.Host,
			Method: req.Method,
			Header: req.Header,
		}
	} else {
		tx = &TransportContext{}
	}
	tx.AddIgnoreHeaderPrefix("accept-encoding", "user-agent", "connection", "content-type")
	return *tx
}

// NewTransportContextFromResponse creates a new TransportContext from a http.Response.
// If `res` is nil, it returns a context with a http.StatusInternalServerError status code.
func NewTransportContextFromResponse(res *http.Response) TransportContext {
	var tx *TransportContext
	if res != nil {
		tx = &TransportContext{
			Header:     res.Header,
			StatusCode: res.StatusCode,
		}
	} else {
		tx = &TransportContext{StatusCode: http.StatusInternalServerError}
	}
	tx.AddIgnoreHeaderPrefix("accept-encoding", "user-agent", "connection", "content-type")
	return *tx
}

// TransportResponseContext allows a Receiver response with http transport specific fields.
type TransportResponseContext struct {
	// Header will be merged with the response headers.
	Header http.Header
}

// AttendToHeaders returns the list of headers that exist in the TransportContext that are not currently in
// tx.IgnoreHeaderPrefix.
func (tx TransportContext) AttendToHeaders() []string {
	a := []string(nil)
	if tx.Header != nil && len(tx.Header) > 0 {
		for k := range tx.Header {
			if tx.shouldIgnoreHeader(k) {
				continue
			}
			a = append(a, k)
		}
	}
	return a
}

func (tx TransportContext) shouldIgnoreHeader(h string) bool {
	for _, v := range tx.IgnoreHeaderPrefixes {
		if strings.HasPrefix(strings.ToLower(h), strings.ToLower(v)) {
			return true
		}
	}
	return false
}

// String generates a pretty-printed version of the resource as a string.
func (tx TransportContext) String() string {
	b := strings.Builder{}

	b.WriteString("Transport Context,\n")

	empty := b.Len()

	if tx.URI != "" {
		b.WriteString("  URI: " + tx.URI + "\n")
	}
	if tx.Host != "" {
		b.WriteString("  Host: " + tx.Host + "\n")
	}

	if tx.Method != "" {
		b.WriteString("  Method: " + tx.Method + "\n")
	}

	if tx.StatusCode != 0 {
		b.WriteString("  StatusCode: " + strconv.Itoa(tx.StatusCode) + "\n")
	}

	if tx.Header != nil && len(tx.Header) > 0 {
		b.WriteString("  Header:\n")
		for _, k := range tx.AttendToHeaders() {
			b.WriteString(fmt.Sprintf("    %s: %s\n", k, tx.Header.Get(k)))
		}
	}

	if b.Len() == empty {
		b.WriteString("  nil\n")
	}

	return b.String()
}

// AddIgnoreHeaderPrefix controls what header key is to be attended to and/or printed.
func (tx *TransportContext) AddIgnoreHeaderPrefix(prefix ...string) {
	if tx.IgnoreHeaderPrefixes == nil {
		tx.IgnoreHeaderPrefixes = []string(nil)
	}
	tx.IgnoreHeaderPrefixes = append(tx.IgnoreHeaderPrefixes, prefix...)
}

// Opaque key type used to store TransportContext
type transportContextKeyType struct{}

var transportContextKey = transportContextKeyType{}

// WithTransportContext return a context with the given TransportContext into the provided context object.
func WithTransportContext(ctx context.Context, tcxt TransportContext) context.Context {
	return context.WithValue(ctx, transportContextKey, tcxt)
}

// TransportContextFrom pulls a TransportContext out of a context. Always
// returns a non-nil object.
func TransportContextFrom(ctx context.Context) TransportContext {
	tctx := ctx.Value(transportContextKey)
	if tctx != nil {
		if tx, ok := tctx.(TransportContext); ok {
			return tx
		}
		if tx, ok := tctx.(*TransportContext); ok {
			return *tx
		}
	}
	return TransportContext{}
}

// Opaque key type used to store Headers
type headerKeyType struct{}

var headerKey = headerKeyType{}

// ContextWithHeader returns a context with a header added to the given context.
// Can be called multiple times to set multiple header key/value pairs.
func ContextWithHeader(ctx context.Context, key, value string) context.Context {
	header := HeaderFrom(ctx)
	header.Add(key, value)
	return context.WithValue(ctx, headerKey, header)
}

// HeaderFrom extracts the header object in the given context. Always returns a non-nil Header.
func HeaderFrom(ctx context.Context) http.Header {
	ch := http.Header{}
	header := ctx.Value(headerKey)
	if header != nil {
		if h, ok := header.(http.Header); ok {
			copyHeaders(h, ch)
		}
	}
	return ch
}

// Opaque key type used to store long poll target.
type longPollTargetKeyType struct{}

var longPollTargetKey = longPollTargetKeyType{}

// WithLongPollTarget returns a new context with the given long poll target.
// `target` should be a full URL and will be injected into the long polling
// http request within StartReceiver.
func ContextWithLongPollTarget(ctx context.Context, target string) context.Context {
	return context.WithValue(ctx, longPollTargetKey, target)
}

// LongPollTargetFrom looks in the given context and returns `target` as a
// parsed url if found and valid, otherwise nil.
func LongPollTargetFrom(ctx context.Context) *url.URL {
	c := ctx.Value(longPollTargetKey)
	if c != nil {
		if s, ok := c.(string); ok && s != "" {
			if target, err := url.Parse(s); err == nil {
				return target
			}
		}
	}
	return nil
}
