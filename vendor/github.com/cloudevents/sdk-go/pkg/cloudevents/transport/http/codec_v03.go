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

// CodecV03 represents a http transport codec that uses CloudEvents spec v0.3
type CodecV03 struct {
	CodecStructured

	DefaultEncoding Encoding
}

// Adheres to Codec
var _ transport.Codec = (*CodecV03)(nil)

// Encode implements Codec.Encode
func (v CodecV03) Encode(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	encoding := v.DefaultEncoding
	strEnc := cecontext.EncodingFrom(ctx)
	if strEnc != "" {
		switch strEnc {
		case Binary:
			encoding = BinaryV03
		case Structured:
			encoding = StructuredV03
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

func (v CodecV03) obsEncode(ctx context.Context, e cloudevents.Event, encoding Encoding) (transport.Message, error) {
	switch encoding {
	case Default:
		fallthrough
	case BinaryV03:
		return v.encodeBinary(ctx, e)
	case StructuredV03:
		return v.encodeStructured(ctx, e)
	case BatchedV03:
		return nil, fmt.Errorf("not implemented")
	default:
		return nil, fmt.Errorf("unknown encoding: %d", encoding)
	}
}

// Decode implements Codec.Decode
func (v CodecV03) Decode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(ctx, CodecObserved{o: reportDecode, c: v.inspectEncoding(ctx, msg).Codec()}) // TODO: inspectEncoding is not free.
	e, err := v.obsDecode(ctx, msg)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func (v CodecV03) obsDecode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	switch v.inspectEncoding(ctx, msg) {
	case BinaryV03:
		return v.decodeBinary(ctx, msg)
	case StructuredV03:
		return v.decodeStructured(ctx, cloudevents.CloudEventsVersionV03, msg)
	case BatchedV03:
		return nil, fmt.Errorf("not implemented")
	default:
		return nil, transport.NewErrMessageEncodingUnknown("v03", TransportName)
	}
}

func (v CodecV03) encodeBinary(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	header, err := v.toHeaders(e.Context.AsV03())
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

func (v CodecV03) toHeaders(ec *cloudevents.EventContextV03) (http.Header, error) {
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
	if ec.SchemaURL != nil {
		h.Set("ce-schemaurl", ec.SchemaURL.String())
	}
	if ec.DataContentType != nil && *ec.DataContentType != "" {
		h.Set("Content-Type", *ec.DataContentType)
	}
	if ec.DataContentEncoding != nil {
		h.Set("ce-datacontentencoding", *ec.DataContentEncoding)
	}

	for k, v := range ec.Extensions {
		k = strings.ToLower(k)
		// Per spec, map-valued extensions are converted to a list of headers as:
		// CE-attrib-key
		switch v.(type) {
		case string:
			h.Set("ce-"+k, v.(string))

		case map[string]interface{}:
			mapVal := v.(map[string]interface{})

			for subkey, subval := range mapVal {
				if subvalstr, ok := v.(string); ok {
					h.Set("ce-"+k+"-"+subkey, subvalstr)
					continue
				}

				encoded, err := json.Marshal(subval)
				if err != nil {
					return nil, err
				}
				h.Set("ce-"+k+"-"+subkey, string(encoded))
			}

		default:
			encoded, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			h.Set("ce-"+k, string(encoded))
		}
	}

	return h, nil
}

func (v CodecV03) decodeBinary(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
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

func (v CodecV03) fromHeaders(h http.Header) (cloudevents.EventContextV03, error) {
	// Normalize headers.
	for k, v := range h {
		ck := textproto.CanonicalMIMEHeaderKey(k)
		if k != ck {
			delete(h, k)
			h[ck] = v
		}
	}

	ec := cloudevents.EventContextV03{}

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

	ec.SchemaURL = types.ParseURLRef(h.Get("ce-schemaurl"))
	h.Del("ce-schemaurl")

	contentType := h.Get("Content-Type")
	if contentType != "" {
		ec.DataContentType = &contentType
	}
	h.Del("Content-Type")

	dataContentEncoding := h.Get("ce-datacontentencoding")
	if dataContentEncoding != "" {
		ec.DataContentEncoding = &dataContentEncoding
	}
	h.Del("ce-datacontentencoding")

	// At this point, we have deleted all the known headers.
	// Everything left is assumed to be an extension.

	extensions := make(map[string]interface{})
	for k, v := range h {
		k = strings.ToLower(k)
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

func (v CodecV03) inspectEncoding(ctx context.Context, msg transport.Message) Encoding {
	version := msg.CloudEventsVersion()
	if version != cloudevents.CloudEventsVersionV03 {
		return Unknown
	}
	m, ok := msg.(*Message)
	if !ok {
		return Unknown
	}
	contentType := m.Header.Get("Content-Type")
	if contentType == cloudevents.ApplicationCloudEventsJSON {
		return StructuredV03
	}
	if contentType == cloudevents.ApplicationCloudEventsBatchJSON {
		return BatchedV03
	}
	return BinaryV03
}
