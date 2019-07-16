package xml

import (
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/observability"
	"strconv"
)

// Decode takes `in` as []byte, or base64 string, normalizes in to unquoted and
// base64 decoded []byte if required, and then attempts to use xml.Unmarshal
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

	b, ok := in.([]byte)
	if !ok {
		var err error
		b, err = xml.Marshal(in)
		if err != nil {
			return fmt.Errorf("[xml] failed to marshal in: %s", err.Error())
		}
	}

	// If the message is encoded as a base64 block as a string, we need to
	// decode that first before trying to unmarshal the bytes
	if len(b) > 1 && (b[0] == byte('"') || (b[0] == byte('\\') && b[1] == byte('"'))) {
		s, err := strconv.Unquote(string(b))
		if err != nil {
			return fmt.Errorf("[xml] failed to unquote quoted data: %s", err.Error())
		}
		if len(s) > 0 && s[0] == '<' {
			// looks like xml, use it
			b = []byte(s)
		} else if len(s) > 0 {
			// looks like base64, decode
			bs, err := base64.StdEncoding.DecodeString(s)
			if err != nil {
				return fmt.Errorf("[xml] failed to decode base64 encoded string: %s", err.Error())
			}
			b = bs
		}
	}

	if err := xml.Unmarshal(b, out); err != nil {
		return fmt.Errorf("[xml] found bytes, but failed to unmarshal: %s %s", err.Error(), string(b))
	}
	return nil
}

// Encode attempts to xml.Marshal `in` into bytes. Encode will inspect `in`
// and returns `in` unmodified if it is detected that `in` is already a []byte;
// Or xml.Marshal errors.
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
	if b, ok := in.([]byte); ok {
		// check to see if it is a pre-encoded byte string.
		if len(b) > 0 && b[0] == byte('"') {
			return b, nil
		}
	}

	return xml.Marshal(in)
}
