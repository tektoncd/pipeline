package types

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"time"
)

// Timestamp wraps time.Time to normalize the time layout to RFC3339. It is
// intended to enforce compliance with the CloudEvents spec for their
// definition of Timestamp. Custom marshal methods are implemented to ensure
// the outbound Timestamp is a string in the RFC3339 layout.
type Timestamp struct {
	time.Time
}

// ParseTimestamp attempts to parse the given time assuming RFC3339 layout
func ParseTimestamp(t string) *Timestamp {
	if t == "" {
		return nil
	}
	timestamp, err := time.Parse(time.RFC3339Nano, t)
	if err != nil {
		return nil
	}
	return &Timestamp{Time: timestamp}
}

// MarshalJSON implements a custom json marshal method used when this type is
// marshaled using json.Marshal.
func (t *Timestamp) MarshalJSON() ([]byte, error) {
	if t == nil || t.IsZero() {
		return []byte(`""`), nil
	}
	rfc3339 := fmt.Sprintf("%q", t.UTC().Format(time.RFC3339Nano))
	return []byte(rfc3339), nil
}

// UnmarshalJSON implements the json unmarshal method used when this type is
// unmarshed using json.Unmarshal.
func (t *Timestamp) UnmarshalJSON(b []byte) error {
	var timestamp string
	if err := json.Unmarshal(b, &timestamp); err != nil {
		return err
	}
	if pt := ParseTimestamp(timestamp); pt != nil {
		*t = *pt
	}
	return nil
}

// MarshalXML implements a custom xml marshal method used when this type is
// marshaled using xml.Marshal.
func (t *Timestamp) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if t == nil || t.IsZero() {
		return e.EncodeElement(nil, start)
	}
	v := t.UTC().Format(time.RFC3339Nano)
	return e.EncodeElement(v, start)
}

// UnmarshalXML implements the xml unmarshal method used when this type is
// unmarshed using xml.Unmarshal.
func (t *Timestamp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var timestamp string
	if err := d.DecodeElement(&timestamp, &start); err != nil {
		return err
	}
	if pt := ParseTimestamp(timestamp); pt != nil {
		*t = *pt
	}
	return nil
}

// String outputs the time using layout RFC3339.
func (t *Timestamp) String() string {
	if t == nil {
		return time.Time{}.UTC().Format(time.RFC3339Nano)
	}

	return t.UTC().Format(time.RFC3339Nano)
}
