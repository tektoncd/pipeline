package text

import (
	"context"
	"github.com/cloudevents/sdk-go/v2/observability"
)

// DecodeObserved calls Decode and records the results.
func DecodeObserved(ctx context.Context, in []byte, out interface{}) error {
	_, r := observability.NewReporter(ctx, reportDecode)
	err := Decode(ctx, in, out)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}

// EncodeObserved calls Encode and records the results.
func EncodeObserved(ctx context.Context, in interface{}) ([]byte, error) {
	_, r := observability.NewReporter(ctx, reportEncode)
	b, err := Encode(ctx, in)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}
