package cloudevents

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/datacodec"
)

// Data is special. Break it out into it's own file.

// SetData implements EventWriter.SetData
func (e *Event) SetData(obj interface{}) error {
	if e.SpecVersion() != CloudEventsVersionV1 {
		return e.legacySetData(obj)
	}

	// Version 1.0 and above.

	// TODO: we will have to be smarter about how data relates to media type.
	//  but the issue is we can not just encode data anymore without understanding
	//  what the encoding will be on the outbound event. Structured will use
	//  data_base64, binary will not (if the transport supports binary mode).

	// TODO: look at content encoding too.

	switch obj.(type) {
	case []byte:
		e.Data = obj
		e.DataEncoded = true
		e.DataBinary = true
	default:
		data, err := datacodec.Encode(context.Background(), e.DataMediaType(), obj)
		if err != nil {
			return err
		}
		e.Data = data
		e.DataEncoded = true
		e.DataBinary = false
	}

	return nil
}

func (e *Event) legacySetData(obj interface{}) error {
	data, err := datacodec.Encode(context.Background(), e.DataMediaType(), obj)
	if err != nil {
		return err
	}
	if e.DeprecatedDataContentEncoding() == Base64 {
		buf := make([]byte, base64.StdEncoding.EncodedLen(len(data)))
		base64.StdEncoding.Encode(buf, data)
		e.Data = string(buf)
	} else {
		e.Data = data
	}
	e.DataEncoded = true
	return nil
}

func (e *Event) DataBytes() ([]byte, error) {
	if !e.DataEncoded {
		if err := e.SetData(e.Data); err != nil {
			return nil, err
		}
	}

	b, ok := e.Data.([]byte)
	if !ok {
		if s, ok := e.Data.(string); ok {
			b = []byte(s)
		} else {
			// No data.
			return []byte(nil), nil
		}
	}
	return b, nil
}

const (
	quotes = `"'`
)

// DataAs attempts to populate the provided data object with the event payload.
// data should be a pointer type.
func (e Event) DataAs(data interface{}) error { // TODO: Clean this function up
	if e.Data == nil {
		return nil
	}
	obj, ok := e.Data.([]byte)
	if !ok {
		if s, ok := e.Data.(string); ok {
			obj = []byte(s)
		} else {
			return errors.New("data was not a byte slice or string")
		}
	}
	if len(obj) == 0 {
		// No data.
		return nil
	}
	if e.Context.DeprecatedGetDataContentEncoding() == Base64 {
		var bs []byte
		// test to see if we need to unquote the data.
		if obj[0] == quotes[0] || obj[0] == quotes[1] {
			str, err := strconv.Unquote(string(obj))
			if err != nil {
				return err
			}
			bs = []byte(str)
		} else {
			bs = obj
		}

		buf := make([]byte, base64.StdEncoding.DecodedLen(len(bs)))
		n, err := base64.StdEncoding.Decode(buf, bs)
		if err != nil {
			return fmt.Errorf("failed to decode data from base64: %s", err.Error())
		}
		obj = buf[:n]
	}

	mediaType := ""
	if e.Context.GetDataContentType() != "" {
		var err error
		mediaType, err = e.Context.GetDataMediaType()
		if err != nil {
			return err
		}
	}
	return datacodec.Decode(context.Background(), mediaType, obj, data)
}
