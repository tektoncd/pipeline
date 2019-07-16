package datacodec

import (
	"context"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec/xml"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
)

// Decoder is the expected function signature for decoding `in` to `out`. What
// `in` is could be decoder dependent. For example, `in` could be bytes, or a
// base64 string.
type Decoder func(in, out interface{}) error

// Encoder is the expected function signature for encoding `in` to bytes.
// Returns an error if the encoder has an issue encoding `in`.
type Encoder func(in interface{}) ([]byte, error)

var decoder map[string]Decoder
var encoder map[string]Encoder

func init() {
	decoder = make(map[string]Decoder, 10)
	encoder = make(map[string]Encoder, 10)

	AddDecoder("", json.Decode)
	AddDecoder("application/json", json.Decode)
	AddDecoder("text/json", json.Decode)
	AddDecoder("application/xml", xml.Decode)
	AddDecoder("text/xml", xml.Decode)

	AddEncoder("", json.Encode)
	AddEncoder("application/json", json.Encode)
	AddEncoder("text/json", json.Encode)
	AddEncoder("application/xml", xml.Encode)
	AddEncoder("text/xml", xml.Encode)
}

// AddDecoder registers a decoder for a given content type. The codecs will use
// these to decode the data payload from a cloudevent.Event object.
func AddDecoder(contentType string, fn Decoder) {
	decoder[contentType] = fn
}

// AddEncoder registers an encoder for a given content type. The codecs will
// use these to encode the data payload for a cloudevent.Event object.
func AddEncoder(contentType string, fn Encoder) {
	encoder[contentType] = fn
}

// Decode looks up and invokes the decoder registered for the given content
// type. An error is returned if no decoder is registered for the given
// content type.
func Decode(contentType string, in, out interface{}) error {
	// TODO: wire in context.
	_, r := observability.NewReporter(context.Background(), reportDecode)
	err := obsDecode(contentType, in, out)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}

func obsDecode(contentType string, in, out interface{}) error {
	if fn, ok := decoder[contentType]; ok {
		return fn(in, out)
	}
	return fmt.Errorf("[decode] unsupported content type: %q", contentType)
}

// Encode looks up and invokes the encoder registered for the given content
// type. An error is returned if no encoder is registered for the given
// content type.
func Encode(contentType string, in interface{}) ([]byte, error) {
	// TODO: wire in context.
	_, r := observability.NewReporter(context.Background(), reportEncode)
	b, err := obsEncode(contentType, in)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}

func obsEncode(contentType string, in interface{}) ([]byte, error) {
	if fn, ok := encoder[contentType]; ok {
		return fn(in)
	}
	return nil, fmt.Errorf("[encode] unsupported content type: %q", contentType)
}
