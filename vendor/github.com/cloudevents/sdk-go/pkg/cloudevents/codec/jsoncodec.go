package codec

import (
	"context"
	"encoding/json"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"strconv"
)

// JsonEncodeV01 takes in a cloudevent.Event and outputs the byte representation of that event using CloudEvents
// version 0.1 structured json formatting rules.
func JsonEncodeV01(e cloudevents.Event) ([]byte, error) {
	_, r := observability.NewReporter(context.Background(), codecObserved{o: reportEncode, v: "v0.1"})
	b, err := obsJsonEncodeV01(e)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}

func obsJsonEncodeV01(e cloudevents.Event) ([]byte, error) {
	ctx := e.Context.AsV01()
	if ctx.ContentType == nil {
		ctx.ContentType = cloudevents.StringOfApplicationJSON()
	}
	data, err := e.DataBytes()
	if err != nil {
		return nil, err
	}
	return jsonEncode(ctx, data)
}

// JsonEncodeV02 takes in a cloudevent.Event and outputs the byte representation of that event using CloudEvents
// version 0.2 structured json formatting rules.
func JsonEncodeV02(e cloudevents.Event) ([]byte, error) {
	_, r := observability.NewReporter(context.Background(), codecObserved{o: reportEncode, v: "v0.2"})
	b, err := obsJsonEncodeV02(e)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}

func obsJsonEncodeV02(e cloudevents.Event) ([]byte, error) {
	ctx := e.Context.AsV02()
	if ctx.ContentType == nil {
		ctx.ContentType = cloudevents.StringOfApplicationJSON()
	}
	data, err := e.DataBytes()
	if err != nil {
		return nil, err
	}
	return jsonEncode(ctx, data)
}

// JsonEncodeV03 takes in a cloudevent.Event and outputs the byte representation of that event using CloudEvents
// version 0.3 structured json formatting rules.
func JsonEncodeV03(e cloudevents.Event) ([]byte, error) {
	_, r := observability.NewReporter(context.Background(), codecObserved{o: reportEncode, v: "v0.3"})
	b, err := obsJsonEncodeV03(e)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}

func obsJsonEncodeV03(e cloudevents.Event) ([]byte, error) {
	ctx := e.Context.AsV03()
	if ctx.DataContentType == nil {
		ctx.DataContentType = cloudevents.StringOfApplicationJSON()
	}

	data, err := e.DataBytes()
	if err != nil {
		return nil, err
	}
	return jsonEncode(ctx, data)
}

func jsonEncode(ctx cloudevents.EventContextReader, data []byte) ([]byte, error) {
	var b map[string]json.RawMessage
	var err error

	if ctx.GetSpecVersion() == cloudevents.CloudEventsVersionV01 {
		b, err = marshalEventLegacy(ctx)
	} else {
		b, err = marshalEvent(ctx, ctx.GetExtensions())
	}
	if err != nil {
		return nil, err
	}

	if data != nil {
		// data is passed in as an encoded []byte. That slice might be any
		// number of things but for json encoding of the envelope all we care
		// is if the payload is either a string or a json object. If it is a
		// json object, it can be inserted into the body without modification.
		// Otherwise we need to quote it if not already quoted.
		mediaType, err := ctx.GetDataMediaType()
		if err != nil {
			return nil, err
		}
		isBase64 := ctx.GetDataContentEncoding() == cloudevents.Base64
		isJson := mediaType == "" || mediaType == cloudevents.ApplicationJSON || mediaType == cloudevents.TextJSON
		// TODO(#60): we do not support json values at the moment, only objects and lists.
		if isJson && !isBase64 {
			b["data"] = data
		} else if data[0] != byte('"') {
			b["data"] = []byte(strconv.QuoteToASCII(string(data)))
		} else {
			// already quoted
			b["data"] = data
		}
	}

	body, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// JsonDecodeV01 takes in the byte representation of a version 0.1 structured json CloudEvent and returns a
// cloudevent.Event or an error if there are parsing errors.
func JsonDecodeV01(body []byte) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(context.Background(), codecObserved{o: reportDecode, v: "v0.1"})
	e, err := obsJsonDecodeV01(body)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func obsJsonDecodeV01(body []byte) (*cloudevents.Event, error) {
	ec := cloudevents.EventContextV01{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, err
	}

	raw := make(map[string]json.RawMessage)

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}
	var data interface{}
	if d, ok := raw["data"]; ok {
		data = []byte(d)
	}

	return &cloudevents.Event{
		Context:     &ec,
		Data:        data,
		DataEncoded: true,
	}, nil
}

// JsonDecodeV02 takes in the byte representation of a version 0.2 structured json CloudEvent and returns a
// cloudevent.Event or an error if there are parsing errors.
func JsonDecodeV02(body []byte) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(context.Background(), codecObserved{o: reportDecode, v: "v0.2"})
	e, err := obsJsonDecodeV02(body)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func obsJsonDecodeV02(body []byte) (*cloudevents.Event, error) {
	ec := cloudevents.EventContextV02{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, err
	}

	raw := make(map[string]json.RawMessage, 0)

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// TODO: could use reflection to get these.
	delete(raw, "specversion")
	delete(raw, "type")
	delete(raw, "source")
	delete(raw, "id")
	delete(raw, "time")
	delete(raw, "schemaurl")
	delete(raw, "contenttype")

	var data interface{}
	if d, ok := raw["data"]; ok {
		data = []byte(d)
	}
	delete(raw, "data")

	if len(raw) > 0 {
		extensions := make(map[string]interface{}, len(raw))
		for k, v := range raw {
			extensions[k] = v
		}
		ec.Extensions = extensions
	}

	return &cloudevents.Event{
		Context:     &ec,
		Data:        data,
		DataEncoded: true,
	}, nil
}

// JsonDecodeV03 takes in the byte representation of a version 0.3 structured json CloudEvent and returns a
// cloudevent.Event or an error if there are parsing errors.
func JsonDecodeV03(body []byte) (*cloudevents.Event, error) {
	_, r := observability.NewReporter(context.Background(), codecObserved{o: reportDecode, v: "v0.3"})
	e, err := obsJsonDecodeV03(body)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return e, err
}

func obsJsonDecodeV03(body []byte) (*cloudevents.Event, error) {
	ec := cloudevents.EventContextV03{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return nil, err
	}

	raw := make(map[string]json.RawMessage)

	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, err
	}

	// TODO: could use reflection to get these.
	delete(raw, "specversion")
	delete(raw, "type")
	delete(raw, "source")
	delete(raw, "subject")
	delete(raw, "id")
	delete(raw, "time")
	delete(raw, "schemaurl")
	delete(raw, "datacontenttype")
	delete(raw, "datacontentencoding")

	var data interface{}
	if d, ok := raw["data"]; ok {
		data = []byte(d)
	}
	delete(raw, "data")

	if len(raw) > 0 {
		extensions := make(map[string]interface{}, len(raw))
		for k, v := range raw {
			extensions[k] = v
		}
		ec.Extensions = extensions
	}

	return &cloudevents.Event{
		Context:     &ec,
		Data:        data,
		DataEncoded: true,
	}, nil
}

func marshalEventLegacy(event interface{}) (map[string]json.RawMessage, error) {
	b, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	brm := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &brm); err != nil {
		return nil, err
	}

	return brm, nil
}

func marshalEvent(event interface{}, extensions map[string]interface{}) (map[string]json.RawMessage, error) {
	b, err := json.Marshal(event)
	if err != nil {
		return nil, err
	}

	brm := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &brm); err != nil {
		return nil, err
	}

	for k, v := range extensions {
		vb, err := json.Marshal(v)
		if err != nil {
			return nil, err
		}
		// Don't overwrite spec keys.
		if _, ok := brm[k]; !ok {
			brm[k] = vb
		}
	}

	return brm, nil
}
