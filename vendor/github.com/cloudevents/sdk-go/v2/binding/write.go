package binding

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event"
)

const (
	SKIP_DIRECT_STRUCTURED_ENCODING = "SKIP_DIRECT_STRUCTURED_ENCODING"
	SKIP_DIRECT_BINARY_ENCODING     = "SKIP_DIRECT_BINARY_ENCODING"
	PREFERRED_EVENT_ENCODING        = "PREFERRED_EVENT_ENCODING"
)

// Invokes the encoders. structuredWriter and binaryWriter could be nil if the protocol doesn't support it.
// transformers can be nil and this function guarantees that they are invoked only once during the encoding process.
//
// Returns:
// * EncodingStructured, nil if message is correctly encoded in structured encoding
// * EncodingBinary, nil if message is correctly encoded in binary encoding
// * EncodingStructured, err if message was structured but error happened during the encoding
// * EncodingBinary, err if message was binary but error happened during the encoding
// * EncodingUnknown, ErrUnknownEncoding if message is not a structured or a binary Message
func DirectWrite(
	ctx context.Context,
	message MessageReader,
	structuredWriter StructuredWriter,
	binaryWriter BinaryWriter,
	transformers ...TransformerFactory,
) (Encoding, error) {
	if structuredWriter != nil && !GetOrDefaultFromCtx(ctx, SKIP_DIRECT_STRUCTURED_ENCODING, false).(bool) {
		// Wrap the transformers in the structured builder
		structuredWriter = TransformerFactories(transformers).StructuredTransformer(structuredWriter)

		// StructuredTransformer could return nil if one of transcoders doesn't support
		// direct structured transcoding
		if structuredWriter != nil {
			if err := message.ReadStructured(ctx, structuredWriter); err == nil {
				return EncodingStructured, nil
			} else if err != ErrNotStructured {
				return EncodingStructured, err
			}
		}
	}

	if binaryWriter != nil && !GetOrDefaultFromCtx(ctx, SKIP_DIRECT_BINARY_ENCODING, false).(bool) {
		binaryWriter = TransformerFactories(transformers).BinaryTransformer(binaryWriter)
		if binaryWriter != nil {
			if err := message.ReadBinary(ctx, binaryWriter); err == nil {
				return EncodingBinary, nil
			} else if err != ErrNotBinary {
				return EncodingBinary, err
			}
		}
	}

	return EncodingUnknown, ErrUnknownEncoding
}

// This is the full algorithm to encode a Message using transformers:
// 1. It first tries direct encoding using DirectWrite
// 2. If no direct encoding is possible, it uses ToEvent to generate an Event representation
// 3. From the Event, the message is encoded back to the provided structured or binary encoders
// You can tweak the encoding process using the context decorators WithForceStructured, WithForceStructured, etc.
// transformers can be nil and this function guarantees that they are invoked only once during the encoding process.
// Returns:
// * EncodingStructured, nil if message is correctly encoded in structured encoding
// * EncodingBinary, nil if message is correctly encoded in binary encoding
// * EncodingUnknown, ErrUnknownEncoding if message.ReadEncoding() == EncodingUnknown
// * _, err if error happened during the encoding
func Write(
	ctx context.Context,
	message MessageReader,
	structuredWriter StructuredWriter,
	binaryWriter BinaryWriter,
	transformers ...TransformerFactory,
) (Encoding, error) {
	enc := message.ReadEncoding()
	var err error
	// Skip direct encoding if the event is an event message
	if enc != EncodingEvent {
		enc, err = DirectWrite(ctx, message, structuredWriter, binaryWriter, transformers...)
		if enc != EncodingUnknown {
			// Message directly encoded, nothing else to do here
			return enc, err
		}
	}

	var e *event.Event
	e, err = ToEvent(ctx, message, transformers...)
	if err != nil {
		return enc, err
	}

	message = (*EventMessage)(e)

	if GetOrDefaultFromCtx(ctx, PREFERRED_EVENT_ENCODING, EncodingBinary).(Encoding) == EncodingStructured {
		if structuredWriter != nil {
			return EncodingStructured, message.ReadStructured(ctx, structuredWriter)
		}
		if binaryWriter != nil {
			return EncodingBinary, message.ReadBinary(ctx, binaryWriter)
		}
	} else {
		if binaryWriter != nil {
			return EncodingBinary, message.ReadBinary(ctx, binaryWriter)
		}
		if structuredWriter != nil {
			return EncodingStructured, message.ReadStructured(ctx, structuredWriter)
		}
	}

	return EncodingUnknown, ErrUnknownEncoding
}

// Skip direct structured to structured encoding during the encoding process
func WithSkipDirectStructuredEncoding(ctx context.Context, skip bool) context.Context {
	return context.WithValue(ctx, SKIP_DIRECT_STRUCTURED_ENCODING, skip)
}

// Skip direct binary to binary encoding during the encoding process
func WithSkipDirectBinaryEncoding(ctx context.Context, skip bool) context.Context {
	return context.WithValue(ctx, SKIP_DIRECT_BINARY_ENCODING, skip)
}

// Define the preferred encoding from event to message during the encoding process
func WithPreferredEventEncoding(ctx context.Context, enc Encoding) context.Context {
	return context.WithValue(ctx, PREFERRED_EVENT_ENCODING, enc)
}

// Force structured encoding during the encoding process
func WithForceStructured(ctx context.Context) context.Context {
	return context.WithValue(context.WithValue(ctx, PREFERRED_EVENT_ENCODING, EncodingStructured), SKIP_DIRECT_BINARY_ENCODING, true)
}

// Force binary encoding during the encoding process
func WithForceBinary(ctx context.Context) context.Context {
	return context.WithValue(context.WithValue(ctx, PREFERRED_EVENT_ENCODING, EncodingBinary), SKIP_DIRECT_STRUCTURED_ENCODING, true)
}

// Get a configuration value from the provided context
func GetOrDefaultFromCtx(ctx context.Context, key string, def interface{}) interface{} {
	if val := ctx.Value(key); val != nil {
		return val
	} else {
		return def
	}
}
