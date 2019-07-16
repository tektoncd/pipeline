package types

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
)

// URLRef is a wrapper to url.URL. It is intended to enforce compliance with
// the CloudEvents spec for their definition of URI-Reference. Custom
// marshal methods are implemented to ensure the outbound URLRef object is
// is a flat string.
type URLRef struct {
	url.URL
}

// ParseURLRef attempts to parse the given string as a URI-Reference.
func ParseURLRef(u string) *URLRef {
	if u == "" {
		return nil
	}
	pu, err := url.Parse(u)
	if err != nil {
		return nil
	}
	return &URLRef{URL: *pu}
}

// MarshalJSON implements a custom json marshal method used when this type is
// marshaled using json.Marshal.
func (u URLRef) MarshalJSON() ([]byte, error) {
	b := fmt.Sprintf("%q", u.String())
	return []byte(b), nil
}

// UnmarshalJSON implements the json unmarshal method used when this type is
// unmarshed using json.Unmarshal.
func (u *URLRef) UnmarshalJSON(b []byte) error {
	var ref string
	if err := json.Unmarshal(b, &ref); err != nil {
		return err
	}
	r := ParseURLRef(ref)
	if r != nil {
		*u = *r
	}
	return nil
}

// MarshalXML implements a custom xml marshal method used when this type is
// marshaled using xml.Marshal.
func (u URLRef) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	v := fmt.Sprintf("%s", u.String())
	return e.EncodeElement(v, start)
}

// UnmarshalXML implements the xml unmarshal method used when this type is
// unmarshed using xml.Unmarshal.
func (u *URLRef) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var ref string
	if err := d.DecodeElement(&ref, &start); err != nil {
		return err
	}
	r := ParseURLRef(ref)
	if r != nil {
		*u = *r
	}
	return nil
}

// String returns the full string representation of the URI-Reference.
func (u *URLRef) String() string {
	if u == nil {
		return ""
	}
	return u.URL.String()
}
