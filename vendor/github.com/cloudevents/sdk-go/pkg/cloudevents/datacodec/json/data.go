package json

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"reflect"
	"strconv"
)

// Decode takes `in` as []byte, or base64 string, normalizes in to unquoted and
// base64 decoded []byte if required, and then attempts to use json.Unmarshal
// to convert those bytes to `out`. Returns and error if this process fails.
func Decode(in, out interface{}) error {
	// TODO: wire in context.
	_, r := observability.NewReporter(context.Background(), reportDecode)
	err := obsDecode(in, out)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return err
}

func obsDecode(in, out interface{}) error {
	if in == nil {
		return nil
	}
	if out == nil {
		return fmt.Errorf("out is nil")
	}

	b, ok := in.([]byte) // TODO: I think there is fancy marshaling happening here. Fix with reflection?
	if !ok {
		var err error
		b, err = json.Marshal(in)
		if err != nil {
			return fmt.Errorf("[json] failed to marshal in: %s", err.Error())
		}
	}

	// TODO: the spec says json could be just data... At the moment we expect wrapped.
	if len(b) > 1 && (b[0] == byte('"') || (b[0] == byte('\\') && b[1] == byte('"'))) {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return fmt.Errorf("[json] failed to unquote in: %s", err.Error())
		}
		if len(s) > 0 && (s[0] == '{' || s[0] == '[') {
			// looks like json, use it
			b = []byte(s)
		}
	}

	if err := json.Unmarshal(b, out); err != nil {
		return fmt.Errorf("[json] found bytes \"%s\", but failed to unmarshal: %s", string(b), err.Error())
	}
	return nil
}

// Encode attempts to json.Marshal `in` into bytes. Encode will inspect `in`
// and returns `in` unmodified if it is detected that `in` is already a []byte;
// Or json.Marshal errors.
func Encode(in interface{}) ([]byte, error) {
	// TODO: wire in context.
	_, r := observability.NewReporter(context.Background(), reportEncode)
	b, err := obsEncode(in)
	if err != nil {
		r.Error()
	} else {
		r.OK()
	}
	return b, err
}

func obsEncode(in interface{}) ([]byte, error) {
	if in == nil {
		return nil, nil
	}

	it := reflect.TypeOf(in)
	switch it.Kind() {
	case reflect.Slice:
		if it.Elem().Kind() == reflect.Uint8 {

			if b, ok := in.([]byte); ok && len(b) > 0 {
				// check to see if it is a pre-encoded byte string.
				if b[0] == byte('"') || b[0] == byte('{') || b[0] == byte('[') {
					return b, nil
				}
			}

		}
	}

	return json.Marshal(in)
}
