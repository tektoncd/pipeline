// Copyright (c) 2014, Greg Roseberry
// All rights reserved.

package null

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"reflect"
)

// String is a nullable string. It supports SQL and JSON
// serialization. It will marshal to null if null. Blank
// string input will be considered null.
type String struct {
	sql.NullString
}

// UnmarshalJSON implements json.Unmarshaler.
// It supports string and null input. Blank string input
// does not produce a null String. It also supports
// unmarshalling a sql.NullString.
func (s *String) UnmarshalJSON(data []byte) error {
	var v interface{}
	err := json.Unmarshal(data, &v)
	if err != nil {
		return err
	}
	switch x := v.(type) {
	case string:
		s.String = x
	case map[string]interface{}:
		err = json.Unmarshal(data, &s.NullString)
	case nil:
		s.Valid = false
		return nil
	default:
		err = fmt.Errorf("json: cannot unmarshal %v into Go value of type null.String", reflect.TypeOf(v).Name())
	}
	s.Valid = err == nil
	return err
}

// IsZero returns true for null strings, for potential
// future omitempty support.
func (s String) IsZero() bool {
	return !s.Valid
}
