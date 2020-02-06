package http

import (
	"context"
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

// CodecV1 represents a http transport codec that uses CloudEvents spec v1.0
type CodecV1 struct {
	CodecStructured

	DefaultEncoding Encoding
}

// Adheres to Codec
var _ transport.Codec = (*CodecV1)(nil)

// Encode implements Codec.Encode
func (v CodecV1) Encode(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	encoding := v.DefaultEncoding
	strEnc := cecontext.EncodingFrom(ctx)
	if strEnc != "" {
		switch strEnc {
		case Binary:
			encoding = BinaryV1
		case Structured:
			encoding = StructuredV1
		}
	}

	_, r := observability.NewReporter(ctx, CodecObserved{o: reportEncode, c: encoding.Codec()})
	m, err := v.obsEncode(ctx, e, encoding)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return m, err
}

func (v CodecV1) obsEncode(ctx context.Context, e cloudevents.Event, encoding Encoding) (transport.Message, error) {
	switch encoding {
	case Default:
		fallthrough
	case BinaryV1:
		return v.encodeBinary(ctx, e)
	case StructuredV1:
		return v.encodeStructured(ctx, e)
	case BatchedV1:
		return nil, fmt.Errorf("not implemented")
	default:
		return nil, fmt.Errorf("unknown encoding: %d", encoding)
	}
}

// Decode implements Codec.Decode
func (v CodecV1) Decode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(ctx, CodecObserved{o: reportDecode, c: v.inspectEncoding(ctx, msg).Codec()}) // TODO: inspectEncoding is not free.
	e, err := v.obsDecode(ctx, msg)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func (v CodecV1) obsDecode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	switch v.inspectEncoding(ctx, msg) {
	case BinaryV1:
		return v.decodeBinary(ctx, msg)
	case StructuredV1:
		return v.decodeStructured(ctx, cloudevents.CloudEventsVersionV1, msg)
	case BatchedV1:
		return nil, fmt.Errorf("not implemented")
	default:
		return nil, transport.NewErrMessageEncodingUnknown("V1", TransportName)
	}
}

func (v CodecV1) encodeBinary(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	header, err := v.toHeaders(e.Context.AsV1())
	if err != nil {
		return nil, err
	}

	body, err := e.DataBytes()
	if err != nil {
		return nil, err
	}

	msg := &Message{
		Header: header,
		Body:   body,
	}

	return msg, nil
}

func (v CodecV1) toHeaders(ec *cloudevents.EventContextV1) (http.Header, error) {
	h := http.Header{}
	h.Set("ce-specversion", ec.SpecVersion)
	h.Set("ce-type", ec.Type)
	h.Set("ce-source", ec.Source.String())
	if ec.Subject != nil {
		h.Set("ce-subject", *ec.Subject)
	}
	h.Set("ce-id", ec.ID)
	if ec.Time != nil && !ec.Time.IsZero() {
		h.Set("ce-time", ec.Time.String())
	}
	if ec.DataSchema != nil {
		h.Set("ce-dataschema", ec.DataSchema.String())
	}
	if ec.DataContentType != nil && *ec.DataContentType != "" {
		h.Set("Content-Type", *ec.DataContentType)
	}

	for k, v := range ec.Extensions {
		k = strings.ToLower(k)
		// Per spec, extensions are strings and converted to a list of headers as:
		// ce-key: value
		cstr, err := types.Format(v)
		if err != nil {
			return h, err
		}
		h.Set("ce-"+k, cstr)
	}

	return h, nil
}

func (v CodecV1) decodeBinary(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
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

func (v CodecV1) fromHeaders(h http.Header) (cloudevents.EventContextV1, error) {
	// Normalize headers.
	for k, v := range h {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if k != ck {
			delete(h, k)
			h[ck] = v
		}
	}

	ec := cloudevents.EventContextV1{}

	ec.SpecVersion = h.Get("ce-specversion")
	h.Del("ce-specversion")

	ec.ID = h.Get("ce-id")
	h.Del("ce-id")

	ec.Type = h.Get("ce-type")
	h.Del("ce-type")

	source := types.ParseURIRef(h.Get("ce-source"))
	if source != nil {
		ec.Source = *source
	}
	h.Del("ce-source")

	subject := h.Get("ce-subject")
	if subject != "" {
		ec.Subject = &subject
	}
	h.Del("ce-subject")

	var err error
	ec.Time, err = types.ParseTimestamp(h.Get("ce-time"))
	if err != nil {
		return ec, err
	}
	h.Del("ce-time")

	ec.DataSchema = types.ParseURI(h.Get("ce-dataschema"))
	h.Del("ce-dataschema")

	contentType := h.Get("Content-Type")
	if contentType != "" {
		ec.DataContentType = &contentType
	}
	h.Del("Content-Type")

	// At this point, we have deleted all the known headers.
	// Everything left is assumed to be an extension.

	extensions := make(map[string]interface{})
	for k := range h {
		k = strings.ToLower(k)
		if len(k) > len("ce-") && strings.EqualFold(k[:len("ce-")], "ce-") {
			ak := strings.ToLower(k[len("ce-"):])
			extensions[ak] = h.Get(k)
		}
	}
	if len(extensions) > 0 {
		ec.Extensions = extensions
	}
	return ec, nil
}

func (v CodecV1) inspectEncoding(ctx context.Context, msg transport.Message) Encoding {
	version := msg.CloudEventsVersion()
	if version != cloudevents.CloudEventsVersionV1 {
		return Unknown
	}
	m, ok := msg.(*Message)
	if !ok {
		return Unknown
	}
	contentType := m.Header.Get("Content-Type")
	if contentType == cloudevents.ApplicationCloudEventsJSON {
		return StructuredV1
	}
	if contentType == cloudevents.ApplicationCloudEventsBatchJSON {
		return BatchedV1
	}
	return BinaryV1
}
