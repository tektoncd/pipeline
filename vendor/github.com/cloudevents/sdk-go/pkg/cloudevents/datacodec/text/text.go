// Text codec converts []byte or string to string and vice-versa.
package text

import (
	"context"
	"fmt"
)

func Decode(_ context.Context, in, out interface{}) error {
	p, _ := out.(*string)
	if p == nil {
		return fmt.Errorf("text.Decode out: want *string, got %T", out)
	}
	switch s := in.(type) {
	case string:
		*p = s
	case []byte:
		*p = string(s)
	case nil: // treat nil like []byte{}
		*p = ""
	default:
		return fmt.Errorf("text.Decode in: want []byte or string, got %T", in)
	}
	return nil
}

func Encode(_ context.Context, in interface{}) ([]byte, error) {
	s, ok := in.(string)
	if !ok {
		return nil, fmt.Errorf("text.Encode in: want string, got %T", in)
	}
	return []byte(s), nil
}
