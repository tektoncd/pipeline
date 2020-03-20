package binding

import (
	"github.com/cloudevents/sdk-go/v2/event"
)

// Implements a transformation process while transferring the event from the Message implementation
// to the provided encoder
//
// A transformer could optionally not provide an implementation for binary and/or structured encodings,
// returning nil to the respective factory method.
type TransformerFactory interface {
	// Can return nil if the transformation doesn't support structured encoding directly
	StructuredTransformer(writer StructuredWriter) StructuredWriter

	// Can return nil if the transformation doesn't support binary encoding directly
	BinaryTransformer(writer BinaryWriter) BinaryWriter

	// Can return nil if the transformation doesn't support events
	EventTransformer() EventTransformer
}

// Utility type alias to manage multiple TransformerFactory
type TransformerFactories []TransformerFactory

func (t TransformerFactories) StructuredTransformer(writer StructuredWriter) StructuredWriter {
	if writer == nil {
		return nil
	}
	res := writer
	for _, b := range t {
		if r := b.StructuredTransformer(res); r != nil {
			res = r
		} else {
			return nil // Structured not supported!
		}
	}
	return res
}

func (t TransformerFactories) BinaryTransformer(writer BinaryWriter) BinaryWriter {
	if writer == nil {
		return nil
	}
	res := writer
	for i := range t {
		b := t[len(t)-i-1]
		if r := b.BinaryTransformer(res); r != nil {
			res = r
		} else {
			return nil // Binary not supported!
		}
	}
	return res
}

func (t TransformerFactories) EventTransformer() EventTransformer {
	return func(e *event.Event) error {
		for _, b := range t {
			f := b.EventTransformer()
			if f != nil {
				err := f(e)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
}

// EventTransformer mutates the provided Event
type EventTransformer func(*event.Event) error
