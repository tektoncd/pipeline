package http

import (
	"context"
	"io"
	nethttp "net/http"
	"strings"

	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/cloudevents/sdk-go/v2/binding/format"
	"github.com/cloudevents/sdk-go/v2/binding/spec"
)

const prefix = "Ce-"

var specs = spec.WithPrefix(prefix)

const ContentType = "Content-Type"
const ContentLength = "Content-Length"

// Message holds the Header and Body of a HTTP Request or Response.
// The Message instance *must* be constructed from NewMessage function.
// This message *cannot* be read several times. In order to read it more times, buffer it using binding/buffering methods
type Message struct {
	Header     nethttp.Header
	BodyReader io.ReadCloser
	OnFinish   func(error) error

	format  format.Format
	version spec.Version
}

// Check if http.Message implements binding.Message
var _ binding.Message = (*Message)(nil)

// NewMessage returns a binding.Message with header and data.
// The returned binding.Message *cannot* be read several times. In order to read it more times, buffer it using binding/buffering methods
func NewMessage(header nethttp.Header, body io.ReadCloser) *Message {
	m := Message{Header: header}
	if body != nil {
		m.BodyReader = body
	}
	if m.format = format.Lookup(header.Get(ContentType)); m.format == nil {
		m.version = specs.Version(m.Header.Get(specs.PrefixedSpecVersionName()))
	}
	return &m
}

// NewMessageFromHttpRequest returns a binding.Message with header and data.
// The returned binding.Message *cannot* be read several times. In order to read it more times, buffer it using binding/buffering methods
func NewMessageFromHttpRequest(req *nethttp.Request) *Message {
	if req == nil {
		return nil
	}
	return NewMessage(req.Header, req.Body)
}

// NewMessageFromHttpResponse returns a binding.Message with header and data.
// The returned binding.Message *cannot* be read several times. In order to read it more times, buffer it using binding/buffering methods
func NewMessageFromHttpResponse(resp *nethttp.Response) *Message {
	if resp == nil {
		return nil
	}
	msg := NewMessage(resp.Header, resp.Body)
	return msg
}

func (m *Message) ReadEncoding() binding.Encoding {
	if m.version != nil {
		return binding.EncodingBinary
	}
	if m.format != nil {
		return binding.EncodingStructured
	}
	return binding.EncodingUnknown
}

func (m *Message) ReadStructured(ctx context.Context, encoder binding.StructuredWriter) error {
	if m.format == nil {
		return binding.ErrNotStructured
	} else {
		return encoder.SetStructuredEvent(ctx, m.format, m.BodyReader)
	}
}

func (m *Message) ReadBinary(ctx context.Context, encoder binding.BinaryWriter) error {
	if m.version == nil {
		return binding.ErrNotBinary
	}

	err := encoder.Start(ctx)
	if err != nil {
		return err
	}

	for k, v := range m.Header {
		if strings.HasPrefix(k, prefix) {
			attr := m.version.Attribute(k)
			if attr != nil {
				err = encoder.SetAttribute(attr, v[0])
			} else {
				err = encoder.SetExtension(strings.ToLower(strings.TrimPrefix(k, prefix)), v[0])
			}
		} else if k == ContentType {
			err = encoder.SetAttribute(m.version.AttributeFromKind(spec.DataContentType), v[0])
		}
		if err != nil {
			return err
		}
	}

	if m.BodyReader != nil {
		err = encoder.SetData(m.BodyReader)
		if err != nil {
			return err
		}
	}

	return encoder.End(ctx)
}

func (m *Message) Finish(err error) error {
	if m.BodyReader != nil {
		_ = m.BodyReader.Close()
	}
	if m.OnFinish != nil {
		return m.OnFinish(err)
	}
	return nil
}
