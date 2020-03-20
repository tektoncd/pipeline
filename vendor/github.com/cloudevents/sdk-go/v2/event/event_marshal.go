package event

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	errors2 "github.com/pkg/errors"

	"github.com/cloudevents/sdk-go/v2/observability"
)

// MarshalJSON implements a custom json marshal method used when this type is
// marshaled using json.Marshal.
func (e Event) MarshalJSON() ([]byte, error) {
	_, r := observability.NewReporter(context.Background(), eventJSONObserved{o: reportMarshal, v: e.SpecVersion()})

	if err := e.Validate(); err != nil {
		r.Error()
		return nil, err
	}

	var b []byte
	var err error

	switch e.SpecVersion() {
	case CloudEventsVersionV03:
		b, err = JsonEncodeLegacy(e)
	case CloudEventsVersionV1:
		b, err = JsonEncode(e)
	default:
		return nil, fmt.Errorf("unnknown spec version: %q", e.SpecVersion())
	}

	// Report the observable
	if err != nil {
		r.Error()
		return nil, err
	} else {
		r.OK()
	}

	return b, nil
}

// UnmarshalJSON implements the json unmarshal method used when this type is
// unmarshaled using json.Unmarshal.
func (e *Event) UnmarshalJSON(b []byte) error {
	raw := make(map[string]json.RawMessage)
	if err := json.Unmarshal(b, &raw); err != nil {
		return err
	}

	version := versionFromRawMessage(raw)

	_, r := observability.NewReporter(context.Background(), eventJSONObserved{o: reportUnmarshal, v: version})

	var err error
	switch version {
	case CloudEventsVersionV03:
		err = e.JsonDecodeV03(b, raw)
	case CloudEventsVersionV1:
		err = e.JsonDecodeV1(b, raw)
	default:
		return fmt.Errorf("unnknown spec version: %q", version)
	}

	// Report the observable
	if err != nil {
		r.Error()
		return err
	} else {
		r.OK()
	}
	return nil
}

func versionFromRawMessage(raw map[string]json.RawMessage) string {
	// v0.2 and after
	if v, ok := raw["specversion"]; ok {
		var version string
		if err := json.Unmarshal(v, &version); err != nil {
			return ""
		}
		return version
	}
	return ""
}

// JsonEncode
func JsonEncode(e Event) ([]byte, error) {
	return jsonEncode(e.Context, e.DataEncoded, e.DataBinary)
}

// JsonEncodeLegacy
func JsonEncodeLegacy(e Event) ([]byte, error) {
	isBase64 := e.Context.DeprecatedGetDataContentEncoding() == Base64
	return jsonEncode(e.Context, e.Data(), isBase64)
}

func jsonEncode(ctx EventContextReader, data []byte, isBase64 bool) ([]byte, error) {
	var b map[string]json.RawMessage
	var err error

	b, err = marshalEvent(ctx, ctx.GetExtensions())
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
		isJson := mediaType == "" || mediaType == ApplicationJSON || mediaType == TextJSON
		// TODO(#60): we do not support json values at the moment, only objects and lists.
		if isJson && !isBase64 {
			b["data"] = data
		} else {
			var dataKey string
			if ctx.GetSpecVersion() == CloudEventsVersionV1 {
				dataKey = "data_base64"
				buf := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
				base64.StdEncoding.Encode(buf, data)
				data = buf
			} else {
				dataKey = "data"
			}
			if data[0] != byte('"') {
				b[dataKey] = []byte(strconv.QuoteToASCII(string(data)))
			} else {
				// already quoted
				b[dataKey] = data
			}
		}
	}

	body, err := json.Marshal(b)
	if err != nil {
		return nil, err
	}

	return body, nil
}

// JsonDecodeV03 takes in the byte representation of a version 0.3 structured json CloudEvent and returns a
// cloudevent.Event or an error if there are parsing errors.
func (e *Event) JsonDecodeV03(body []byte, raw map[string]json.RawMessage) error {
	ec := EventContextV03{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return err
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

	var data []byte
	if d, ok := raw["data"]; ok {
		data = d
	}
	delete(raw, "data")

	if len(raw) > 0 {
		extensions := make(map[string]interface{}, len(raw))
		ec.Extensions = extensions
		for k, v := range raw {
			k = strings.ToLower(k)
			var tmp interface{}
			if err := json.Unmarshal(v, &tmp); err != nil {
				return err
			}
			if err := ec.SetExtension(k, tmp); err != nil {
				return errors2.Wrap(err, "Cannot set extension with key "+k)
			}
		}
	}

	e.Context = &ec
	e.DataEncoded = data

	return nil
}

// JsonDecodeV1 takes in the byte representation of a version 1.0 structured json CloudEvent and returns a
// cloudevent.Event or an error if there are parsing errors.
func (e *Event) JsonDecodeV1(body []byte, raw map[string]json.RawMessage) error {
	ec := EventContextV1{}
	if err := json.Unmarshal(body, &ec); err != nil {
		return err
	}

	delete(raw, "specversion")
	delete(raw, "type")
	delete(raw, "source")
	delete(raw, "subject")
	delete(raw, "id")
	delete(raw, "time")
	delete(raw, "dataschema")
	delete(raw, "datacontenttype")

	var data []byte
	if d, ok := raw["data"]; ok {
		data = d
	}
	delete(raw, "data")

	var dataBase64 []byte
	if d, ok := raw["data_base64"]; ok {
		var tmp []byte
		if err := json.Unmarshal(d, &tmp); err != nil {
			return err
		}
		dataBase64 = tmp
	}
	delete(raw, "data_base64")

	if len(raw) > 0 {
		extensions := make(map[string]interface{}, len(raw))
		ec.Extensions = extensions
		for k, v := range raw {
			k = strings.ToLower(k)
			var tmp interface{}
			if err := json.Unmarshal(v, &tmp); err != nil {
				return err
			}
			if err := ec.SetExtension(k, tmp); err != nil {
				return errors2.Wrap(err, "Cannot set extension with key "+k)
			}
		}
	}

	e.Context = &ec
	if data != nil && dataBase64 != nil {
		return errors.New("parsing error: JSON decoder found both 'data', and 'data_base64' in JSON payload")
	}
	if data != nil {
		e.DataEncoded = data
		e.DataBinary = false
	} else if dataBase64 != nil {
		e.DataEncoded = dataBase64
		e.DataBinary = true
	}

	return nil
}

func marshalEvent(eventCtx EventContextReader, extensions map[string]interface{}) (map[string]json.RawMessage, error) {
	b, err := json.Marshal(eventCtx)
	if err != nil {
		return nil, err
	}

	brm := map[string]json.RawMessage{}
	if err := json.Unmarshal(b, &brm); err != nil {
		return nil, err
	}

	sv, err := json.Marshal(eventCtx.GetSpecVersion())
	if err != nil {
		return nil, err
	}

	brm["specversion"] = sv

	for k, v := range extensions {
		k = strings.ToLower(k)
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
