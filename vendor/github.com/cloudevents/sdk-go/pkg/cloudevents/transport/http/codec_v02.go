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

// CodecV02 represents a http transport codec that uses CloudEvents spec v0.2
type CodecV02 struct {
	CodecStructured

	DefaultEncoding Encoding
}

// Adheres to Codec
var _ transport.Codec = (*CodecV02)(nil)

// Encode implements Codec.Encode
func (v CodecV02) Encode(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	encoding := v.DefaultEncoding
	strEnc := cecontext.EncodingFrom(ctx)
	if strEnc != "" {
		switch strEnc {
		case Binary:
			encoding = BinaryV02
		case Structured:
			encoding = StructuredV02
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

func (v CodecV02) obsEncode(ctx context.Context, e cloudevents.Event, encoding Encoding) (transport.Message, error) {
	switch encoding {
	case Default:
		fallthrough
	case BinaryV02:
		return v.encodeBinary(ctx, e)
	case StructuredV02:
		return v.encodeStructured(ctx, e)
	default:
		return nil, fmt.Errorf("unknown encoding: %d", encoding)
	}
}

// Decode implements Codec.Decode
func (v CodecV02) Decode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(ctx, CodecObserved{o: reportDecode, c: v.inspectEncoding(ctx, msg).Codec()}) // TODO: inspectEncoding is not free.
	e, err := v.obsDecode(ctx, msg)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func (v CodecV02) obsDecode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	switch v.inspectEncoding(ctx, msg) {
	case BinaryV02:
		return v.decodeBinary(ctx, msg)
	case StructuredV02:
		return v.decodeStructured(ctx, cloudevents.CloudEventsVersionV02, msg)
	default:
		return nil, transport.NewErrMessageEncodingUnknown("v02", TransportName)
	}
}

func (v CodecV02) encodeBinary(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	header, err := v.toHeaders(e.Context.AsV02())
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

func (v CodecV02) toHeaders(ec *cloudevents.EventContextV02) (http.Header, error) {
	h := http.Header{}
	h.Set("ce-specversion", ec.SpecVersion)
	h.Set("ce-type", ec.Type)
	h.Set("ce-source", ec.Source.String())
	h.Set("ce-id", ec.ID)
	if ec.Time != nil && !ec.Time.IsZero() {
		h.Set("ce-time", ec.Time.String())
	}
	if ec.SchemaURL != nil {
		h.Set("ce-schemaurl", ec.SchemaURL.String())
	}
	if ec.ContentType != nil && *ec.ContentType != "" {
		h.Set("Content-Type", *ec.ContentType)
	}
	for k, v := range ec.Extensions {
		// Per spec, map-valued extensions are converted to a list of headers as:
		// CE-attrib-key
		if mapVal, ok := v.(map[string]interface{}); ok {
			for subkey, subval := range mapVal {
				encoded, err := json.Marshal(subval)
				if err != nil {
					return nil, err
				}
				h.Set("ce-"+k+"-"+subkey, string(encoded))
			}
			continue
		}
		encoded, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		h.Set("ce-"+k, string(encoded))
	}

	return h, nil
}

func (v CodecV02) decodeBinary(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
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

func (v CodecV02) fromHeaders(h http.Header) (cloudevents.EventContextV02, error) {
	// Normalize headers.
	for k, v := range h {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if k != ck {
			delete(h, k)
			h[ck] = v
		}
	}

	ec := cloudevents.EventContextV02{}

	ec.SpecVersion = h.Get("ce-specversion")
	h.Del("ce-specversion")

	ec.ID = h.Get("ce-id")
	h.Del("ce-id")

	ec.Type = h.Get("ce-type")
	h.Del("ce-type")

	source := types.ParseURLRef(h.Get("ce-source"))
	if source != nil {
		ec.Source = *source
	}
	h.Del("ce-source")

	var err error
	ec.Time, err = types.ParseTimestamp(h.Get("ce-time"))
	if err != nil {
		return ec, err
	}
	h.Del("ce-time")

	ec.SchemaURL = types.ParseURLRef(h.Get("ce-schemaurl"))
	h.Del("ce-schemaurl")

	contentType := h.Get("Content-Type")
	if contentType != "" {
		ec.ContentType = &contentType
	}
	h.Del("Content-Type")

	// At this point, we have deleted all the known headers.
	// Everything left is assumed to be an extension.

	extensions := make(map[string]interface{})
	for k, v := range h {
		if len(k) > len("ce-") && strings.EqualFold(k[:len("ce-")], "ce-") {
			ak := strings.ToLower(k[len("ce-"):])
			if i := strings.Index(ak, "-"); i > 0 {
				// attrib-key
				attrib := ak[:i]
				key := ak[(i + 1):]
				if xv, ok := extensions[attrib]; ok {
					if m, ok := xv.(map[string]interface{}); ok {
						m[key] = v
						continue
					}
					// TODO: revisit how we want to bubble errors up.
					return ec, fmt.Errorf("failed to process map type extension")
				} else {
					m := make(map[string]interface{})
					m[key] = v
					extensions[attrib] = m
				}
			} else {
				// key
				var tmp interface{}
				if err := json.Unmarshal([]byte(v[0]), &tmp); err == nil {
					extensions[ak] = tmp
				} else {
					// If we can't unmarshal the data, treat it as a string.
					extensions[ak] = v[0]
				}
			}
		}
	}
	if len(extensions) > 0 {
		ec.Extensions = extensions
	}
	return ec, nil
}

func (v CodecV02) inspectEncoding(ctx context.Context, msg transport.Message) Encoding {
	version := msg.CloudEventsVersion()
	if version != cloudevents.CloudEventsVersionV02 {
		return Unknown
	}
	m, ok := msg.(*Message)
	if !ok {
		return Unknown
	}
	contentType := m.Header.Get("Content-Type")
	if contentType == cloudevents.ApplicationCloudEventsJSON {
		return StructuredV02
	}
	return BinaryV02
}
