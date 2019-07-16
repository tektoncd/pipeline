package http

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/codec"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"net/http"
	"net/textproto"
	"strings"
)

// CodecV01 represents a http transport codec that uses CloudEvents spec v0.3
type CodecV01 struct {
	Encoding Encoding
}

// Adheres to Codec
var _ transport.Codec = (*CodecV01)(nil)

// Encode implements Codec.Encode
func (v CodecV01) Encode(e cloudevents.Event) (transport.Message, error) {
	// TODO: wire context
	_, r := observability.NewReporter(context.Background(), CodecObserved{o: reportEncode, c: v.Encoding.Codec()})
	m, err := v.obsEncode(e)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return m, err
}

func (v CodecV01) obsEncode(e cloudevents.Event) (transport.Message, error) {
	switch v.Encoding {
	case Default:
		fallthrough
	case BinaryV01:
		return v.encodeBinary(e)
	case StructuredV01:
		return v.encodeStructured(e)
	default:
		return nil, fmt.Errorf("unknown encoding: %d", v.Encoding)
	}
}

// Decode implements Codec.Decode
func (v CodecV01) Decode(msg transport.Message) (*cloudevents.Event, error) {
	// TODO: wire context
	_, r := observability.NewReporter(context.Background(), CodecObserved{o: reportDecode, c: v.inspectEncoding(msg).Codec()}) // TODO: inspectEncoding is not free.
	e, err := v.obsDecode(msg)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func (v CodecV01) obsDecode(msg transport.Message) (*cloudevents.Event, error) {
	switch v.inspectEncoding(msg) {
	case BinaryV01:
		return v.decodeBinary(msg)
	case StructuredV01:
		return v.decodeStructured(msg)
	default:
		return nil, fmt.Errorf("unknown encoding")
	}
}

func (v CodecV01) encodeBinary(e cloudevents.Event) (transport.Message, error) {
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
		h["CE-SchemaURL"] = []string{ec.SchemaURL.String()}
	}
	if ec.ContentType != nil {
		h.Set("Content-Type", *ec.ContentType)
	} else if v.Encoding == Default || v.Encoding == BinaryV01 {
		// in binary v0.1, the Content-Type header is tied to ec.ContentType
		// This was later found to be an issue with the spec, but yolo.
		// TODO: not sure what the default should be?
		h.Set("Content-Type", cloudevents.ApplicationJSON)
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

func (v CodecV01) encodeStructured(e cloudevents.Event) (transport.Message, error) {
	header := http.Header{}
	header.Set("Content-Type", cloudevents.ApplicationCloudEventsJSON)

	body, err := codec.JsonEncodeV01(e)
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Header: header,
		Body:   body,
	}

	return msg, nil
}

func (v CodecV01) decodeBinary(msg transport.Message) (*cloudevents.Event, error) {
	m, ok := msg.(*Message)
	if !ok {
		return nil, fmt.Errorf("failed to convert transport.Message to http.Message")
	}
	ctx, err := v.fromHeaders(m.Header)
	if err != nil {
		return nil, err
	}
	var body interface{}
	if len(m.Body) > 0 {
		body = m.Body
	}
	return &cloudevents.Event{
		Context:     &ctx,
		Data:        body,
		DataEncoded: true,
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
	ec.EventTime = types.ParseTimestamp(h.Get("CE-EventTime"))
	h.Del("CE-EventTime")
	etv := h.Get("CE-EventTypeVersion")
	h.Del("CE-EventTypeVersion")
	if etv != "" {
		ec.EventTypeVersion = &etv
	}
	ec.SchemaURL = types.ParseURLRef(h.Get("CE-SchemaURL"))
	h.Del("CE-SchemaURL")
	et := h.Get("Content-Type")
	ec.ContentType = &et

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

func (v CodecV01) decodeStructured(msg transport.Message) (*cloudevents.Event, error) {
	m, ok := msg.(*Message)
	if !ok {
		return nil, fmt.Errorf("failed to convert transport.Message to http.Message")
	}
	return codec.JsonDecodeV01(m.Body)
}

func (v CodecV01) inspectEncoding(msg transport.Message) Encoding {
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
