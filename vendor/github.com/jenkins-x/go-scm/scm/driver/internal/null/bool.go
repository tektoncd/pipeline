// Copyright (c) 2014, Greg Roseberry
// All rights reserved.

package null

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
)

// Bool is a nullable bool. It does not consider false values
// to be null. It will decode to null, not false, if null.
type Bool struct {
	sql.NullBool
}

// UnmarshalJSON implements json.Unmarshaler. It supports
// number and null input. 0 will not be considered a null
// Bool. It also supports unmarshalling a sql.NullBool.
func (b *Bool) UnmarshalJSON(data []byte) error {
	var v interface{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	switch x := v.(type) {
	case bool:
		b.Bool = x
	case map[string]interface{}:
		err = json.Unmarshal(data, &b.NullBool)
	case nil:
		b.Valid = false
		return nil
	default:
		err = fmt.Errorf("json: cannot unmarshal %v into Go value of type null.Bool", reflect.TypeOf(v).Name())
	}
	b.Valid = err == nil
	return err
}

// IsZero returns true for invalid Bools, for future omitempty
// support (Go 1.4?). A non-null Bool with a 0 value will not be
// considered zero.
func (b Bool) IsZero() bool {
	return !b.Valid
}
