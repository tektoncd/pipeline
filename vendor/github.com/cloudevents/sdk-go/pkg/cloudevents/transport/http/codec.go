package http

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/transport"
)

// Codec is the wrapper for all versions of codecs supported by the http
// transport.
type Codec struct {
	// Encoding is the setting to inform the DefaultEncodingSelectionFn for
	// selecting a codec.
	Encoding Encoding

	// DefaultEncodingSelectionFn allows for encoding selection strategies to be injected.
	DefaultEncodingSelectionFn EncodingSelector

	v01 *CodecV01
	v02 *CodecV02
	v03 *CodecV03
	v1  *CodecV1

	_v01 sync.Once
	_v02 sync.Once
	_v03 sync.Once
	_v1  sync.Once
}

// Adheres to Codec
var _ transport.Codec = (*Codec)(nil)

func (c *Codec) loadCodec(encoding Encoding) (transport.Codec, error) {
	switch encoding {
	case Default:
		fallthrough
	case BinaryV01, StructuredV01:
		c._v01.Do(func() {
			c.v01 = &CodecV01{DefaultEncoding: c.Encoding}
		})
		return c.v01, nil
	case BinaryV02, StructuredV02:
		c._v02.Do(func() {
			c.v02 = &CodecV02{DefaultEncoding: c.Encoding}
		})
		return c.v02, nil
	case BinaryV03, StructuredV03, BatchedV03:
		c._v03.Do(func() {
			c.v03 = &CodecV03{DefaultEncoding: c.Encoding}
		})
		return c.v03, nil
	case BinaryV1, StructuredV1, BatchedV1:
		c._v1.Do(func() {
			c.v1 = &CodecV1{DefaultEncoding: c.Encoding}
		})
		return c.v1, nil
	}
	return nil, fmt.Errorf("unknown encoding: %s", encoding)
}

// Encode encodes the provided event into a transport message.
func (c *Codec) Encode(ctx context.Context, e cloudevents.Event) (transport.Message, error) {
	encoding := c.Encoding
	if encoding == Default && c.DefaultEncodingSelectionFn != nil {
		encoding = c.DefaultEncodingSelectionFn(ctx, e)
	}
	codec, err := c.loadCodec(encoding)
	if err != nil {
		return nil, err
	}
	ctx = cecontext.WithEncoding(ctx, encoding.Name())
	return codec.Encode(ctx, e)
}

// Decode converts a provided transport message into an Event, or error.
func (c *Codec) Decode(ctx context.Context, msg transport.Message) (*cloudevents.Event, error) {
	codec, err := c.loadCodec(c.inspectEncoding(ctx, msg))
	if err != nil {
		return nil, err
	}
	event, err := codec.Decode(ctx, msg)
	if err != nil {
		return nil, err
	}
	return c.convertEvent(event)
}

// Give the context back as the user expects
func (c *Codec) convertEvent(event *cloudevents.Event) (*cloudevents.Event, error) {
	if event == nil {
		return nil, errors.New("event is nil, can not convert")
	}

	switch c.Encoding {
	case Default:
		return event, nil
	case BinaryV01, StructuredV01:
		ca := event.Context.AsV01()
		event.Context = ca
		return event, nil
	case BinaryV02, StructuredV02:
		ca := event.Context.AsV02()
		event.Context = ca
		return event, nil
	case BinaryV03, StructuredV03, BatchedV03:
		ca := event.Context.AsV03()
		event.Context = ca
		return event, nil
	case BinaryV1, StructuredV1, BatchedV1:
		ca := event.Context.AsV1()
		event.Context = ca
		return event, nil
	default:
		return nil, fmt.Errorf("unknown encoding: %s", c.Encoding)
	}
}

func (c *Codec) inspectEncoding(ctx context.Context, msg transport.Message) Encoding {
	// Try v1.0.
	_, _ = c.loadCodec(BinaryV1)
	encoding := c.v1.inspectEncoding(ctx, msg)
	if encoding != Unknown {
		return encoding
	}

	// Try v0.3.
	_, _ = c.loadCodec(BinaryV03)
	encoding = c.v03.inspectEncoding(ctx, msg)
	if encoding != Unknown {
		return encoding
	}

	// Try v0.2.
	_, _ = c.loadCodec(BinaryV02)
	encoding = c.v02.inspectEncoding(ctx, msg)
	if encoding != Unknown {
		return encoding
	}

	// Try v0.1 first.
	_, _ = c.loadCodec(BinaryV01)
	encoding = c.v01.inspectEncoding(ctx, msg)
	if encoding != Unknown {
		return encoding
	}

	// We do not understand the message encoding.
	return Unknown
}
