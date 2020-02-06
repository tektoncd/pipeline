package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/textproto"
	"strings"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
)

// CodecV01 represents a http transport codec that uses CloudEvents spec v0.1
type CodecV01 struct {
	CodecStructured

	DefaultEncoding Encoding
}

// Adheres to Codec
var _ transport.Codec = (*CodecV01)(nil)

// Encode implements Codec.Encode
func (v CodecV01) Encode(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	encoding := v.DefaultEncoding
	strEnc := cecontext.EncodingFrom(ctx)
	if strEnc != "" {
		switch strEnc {
		case Binary:
			encoding = BinaryV01
		case Structured:
			encoding = StructuredV01
		}
	}

	_, r := observability.NewReporter(context.Background(), CodecObserved{o: reportEncode, c: encoding.Codec()})
	m, err := v.obsEncode(ctx, e, encoding)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return m, err
}

func (v CodecV01) obsEncode(ctx context.Context, e cloudevents.Event, encoding Encoding) (transport.Message, error) {
	switch encoding {
	case Default:
		fallthrough
	case BinaryV01:
		return v.encodeBinary(ctx, e)
	case StructuredV01:
		return v.encodeStructured(ctx, e)
	default:
		return nil, fmt.Errorf("unknown encoding: %d", encoding)
	}
}

// Decode implements Codec.Decode
func (v CodecV01) Decode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(ctx, CodecObserved{o: reportDecode, c: v.inspectEncoding(ctx, msg).Codec()}) // TODO: inspectEncoding is not free.
	e, err := v.obsDecode(ctx, msg)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func (v CodecV01) obsDecode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	switch v.inspectEncoding(ctx, msg) {
	case BinaryV01:
		return v.decodeBinary(ctx, msg)
	case StructuredV01:
		return v.decodeStructured(ctx, cloudevents.CloudEventsVersionV01, msg)
	default:
		return nil, transport.NewErrMessageEncodingUnknown("v01", TransportName)
	}
}

func (v CodecV01) encodeBinary(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	header, err := v.toHeaders(e.Context.AsV01())
	if err != nil {
		return nil, err
	}

	body, err := e.DataBytes()
	if err != nil {
		panic("encode")
	}

	msg := &Message{
		Header: header,
		Body:   body,
	}

	return msg, nil
}

func (v CodecV01) toHeaders(ec *cloudevents.EventContextV01) (http.Header, error) {
	// Preserve case in v0.1, even though HTTP headers are case-insensitive.
	h := http.Header{}
	h["CE-CloudEventsVersion"] = []string{ec.CloudEventsVersion}
	h["CE-EventID"] = []string{ec.EventID}
	h["CE-EventType"] = []string{ec.EventType}
	h["CE-Source"] = []string{ec.Source.String()}
	if ec.EventTime != nil && !ec.EventTime.IsZero() {
		h["CE-EventTime"] = []string{ec.EventTime.String()}
	}
	if ec.EventTypeVersion != nil {
		h["CE-EventTypeVersion"] = []string{*ec.EventTypeVersion}
	}
	if ec.SchemaURL != nil {
		h["CE-DataSchema"] = []string{ec.SchemaURL.String()}
	}
	if ec.ContentType != nil && *ec.ContentType != "" {
		h.Set("Content-Type", *ec.ContentType)
	}

	// Regarding Extensions, v0.1 Spec says the following:
	// * Each map entry name MUST be prefixed with "CE-X-"
	// * Each map entry name's first character MUST be capitalized
	for k, v := range ec.Extensions {
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		h["CE-X-"+strings.Title(k)] = []string{string(encoded)}
	}
	return h, nil
}

func (v CodecV01) decodeBinary(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	m, ok := msg.(*Message)
	if !ok {
		return nil, fmt.Errorf("failed to convert transport.Message to http.Message")
	}
	ca, err := v.fromHeaders(m.Header)
	if err != nil {
		return nil, err
	}
	var body interface{}
	if len(m.Body) > 0 {
		body = m.Body
	}
	return &cloudevents.Event{
		Context:     &ca,
		Data:        body,
		DataEncoded: body != nil,
	}, nil
}

func (v CodecV01) fromHeaders(h http.Header) (cloudevents.EventContextV01, error) {
	// Normalize headers.
	for k, v := range h {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if k != ck {
			h[ck] = v
		}
	}

	ec := cloudevents.EventContextV01{}
	ec.CloudEventsVersion = h.Get("CE-CloudEventsVersion")
	h.Del("CE-CloudEventsVersion")
	ec.EventID = h.Get("CE-EventID")
	h.Del("CE-EventID")
	ec.EventType = h.Get("CE-EventType")
	h.Del("CE-EventType")
	source := types.ParseURLRef(h.Get("CE-Source"))
	h.Del("CE-Source")
	if source != nil {
		ec.Source = *source
	}
	var err error
	ec.EventTime, err = types.ParseTimestamp(h.Get("CE-EventTime"))
	if err != nil {
		return ec, err
	}
	h.Del("CE-EventTime")
	etv := h.Get("CE-EventTypeVersion")
	h.Del("CE-EventTypeVersion")
	if etv != "" {
		ec.EventTypeVersion = &etv
	}
	ec.SchemaURL = types.ParseURLRef(h.Get("CE-DataSchema"))
	h.Del("CE-DataSchema")
	et := h.Get("Content-Type")
	if et != "" {
		ec.ContentType = &et
	}

	extensions := make(map[string]interface{})
	for k, v := range h {
		if len(k) > len("CE-X-") && strings.EqualFold(k[:len("CE-X-")], "CE-X-") {
			key := k[len("CE-X-"):]
			var tmp interface{}
			if err := json.Unmarshal([]byte(v[0]), &tmp); err == nil {
				extensions[key] = tmp
			} else {
				// If we can't unmarshal the data, treat it as a string.
				extensions[key] = v[0]
			}
			h.Del(k)
		}
	}
	if len(extensions) > 0 {
		ec.Extensions = extensions
	}
	return ec, nil
}

func (v CodecV01) inspectEncoding(ctx context.Context, msg transport.Message) Encoding {
	version := msg.CloudEventsVersion()
	if version != cloudevents.CloudEventsVersionV01 {
		return Unknown
	}
	m, ok := msg.(*Message)
	if !ok {
		return Unknown
	}
	contentType := m.Header.Get("Content-Type")
	if contentType == cloudevents.ApplicationCloudEventsJSON {
		return StructuredV01
	}
	return BinaryV01
}
