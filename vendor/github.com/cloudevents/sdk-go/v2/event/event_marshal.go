package event

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

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
		return nil, ValidationError{"specversion": fmt.Errorf("unknown : %q", e.SpecVersion())}
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
		return ValidationError{"specversion": fmt.Errorf("unknown : %q", version)}
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

// JsonEncode encodes an event to JSON
func JsonEncode(e Event) ([]byte, error) {
	return jsonEncode(e.Context, e.DataEncoded, e.DataBase64)
}

// JsonEncodeLegacy performs legacy JSON encoding
func JsonEncodeLegacy(e Event) ([]byte, error) {
	isBase64 := e.Context.DeprecatedGetDataContentEncoding() == Base64
	return jsonEncode(e.Context, e.DataEncoded, isBase64)
}

func jsonEncode(ctx EventContextReader, data []byte, shouldEncodeToBase64 bool) ([]byte, error) {
	var b map[string]json.RawMessage
	var err error

	b, err = marshalEvent(ctx, ctx.GetExtensions())
	if err != nil {
		return nil, err
	}

	if data != nil {
		// data here is a serialized version of whatever payload.
		// If we need to write the payload as base64, shouldEncodeToBase64 is true.
		mediaType, err := ctx.GetDataMediaType()
		if err != nil {
			return nil, err
		}
		isJson := mediaType == "" || mediaType == ApplicationJSON || mediaType == TextJSON
		// If isJson and no encoding to base64, we don't need to perform additional steps
		if isJson && !shouldEncodeToBase64 {
			b["data"] = data
		} else {
			var dataKey = "data"
			if ctx.GetSpecVersion() == CloudEventsVersionV1 && shouldEncodeToBase64 {
				dataKey = "data_base64"
			}
			var dataPointer []byte
			if shouldEncodeToBase64 {
				dataPointer, err = json.Marshal(data)
			} else {
				dataPointer, err = json.Marshal(string(data))
			}
			if err != nil {
				return nil, err
			}

			b[dataKey] = dataPointer
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

		// Decode the Base64 if we have a base64 payload
		if ec.DeprecatedGetDataContentEncoding() == Base64 {
			var tmp []byte
			if err := json.Unmarshal(d, &tmp); err != nil {
				return err
			}
			e.DataBase64 = true
			e.DataEncoded = tmp
		} else {
			if ec.DataContentType != nil {
				ct := *ec.DataContentType
				if ct != ApplicationJSON && ct != TextJSON {
					var dataStr string
					err := json.Unmarshal(d, &dataStr)
					if err != nil {
						return err
					}

					data = []byte(dataStr)
				}
			}
			e.DataEncoded = data
			e.DataBase64 = false
		}
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
				return fmt.Errorf("%w: Cannot set extension with key %s", err, k)
			}
		}
	}

	e.Context = &ec

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
		if ec.DataContentType != nil {
			ct := *ec.DataContentType
			if ct != ApplicationJSON && ct != TextJSON {
				var dataStr string
				err := json.Unmarshal(d, &dataStr)
				if err != nil {
					return err
				}

				data = []byte(dataStr)
			}
		}
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

	if data != nil && dataBase64 != nil {
		return ValidationError{"data": fmt.Errorf("found both 'data', and 'data_base64' in JSON payload")}
	}
	if data != nil {
		e.DataEncoded = data
		e.DataBase64 = false
	} else if dataBase64 != nil {
		e.DataEncoded = dataBase64
		e.DataBase64 = true
	}

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
				return fmt.Errorf("%w: Cannot set extension with key %s", err, k)
			}
		}
	}

	e.Context = &ec

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
