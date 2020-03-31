package datacodec

import (
	"context"

	"github.com/cloudevents/sdk-go/v2/event/datacodec/json"
	"github.com/cloudevents/sdk-go/v2/event/datacodec/text"
	"github.com/cloudevents/sdk-go/v2/event/datacodec/xml"
	"github.com/cloudevents/sdk-go/v2/observability"
)

func SetObservedCodecs() {
	AddDecoder("", json.DecodeObserved)
	AddDecoder("application/json", json.DecodeObserved)
	AddDecoder("text/json", json.DecodeObserved)
	AddDecoder("application/xml", xml.DecodeObserved)
	AddDecoder("text/xml", xml.DecodeObserved)
	AddDecoder("text/plain", text.DecodeObserved)

	AddEncoder("", json.Encode)
	AddEncoder("application/json", json.EncodeObserved)
	AddEncoder("text/json", json.EncodeObserved)
	AddEncoder("application/xml", xml.EncodeObserved)
	AddEncoder("text/xml", xml.EncodeObserved)
	AddEncoder("text/plain", text.EncodeObserved)
}

// DecodeObserved calls Decode and records the result.
func DecodeObserved(ctx context.Context, contentType string, in []byte, out interface{}) error {
	_, r := observability.NewReporter(ctx, reportDecode)
	err := Decode(ctx, contentType, in, out)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}

// EncodeObserved calls Encode and records the result.
func EncodeObserved(ctx context.Context, contentType string, in interface{}) ([]byte, error) {
	_, r := observability.NewReporter(ctx, reportEncode)
	b, err := Encode(ctx, contentType, in)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}
