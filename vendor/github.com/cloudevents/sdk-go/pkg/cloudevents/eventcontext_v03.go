package cloudevents

import (
	"encoding/json"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"sort"
	"strings"
)

// WIP: AS OF FEB 19, 2019

const (
	// CloudEventsVersionV03 represents the version 0.3 of the CloudEvents spec.
	CloudEventsVersionV03 = "0.3"
)

// EventContextV03 represents the non-data attributes of a CloudEvents v0.3
// event.
type EventContextV03 struct {
	// SpecVersion - The version of the CloudEvents specification used by the event.
	SpecVersion string `json:"specversion"`
	// Type - The type of the occurrence which has happened.
	Type string `json:"type"`
	// Source - A URI describing the event producer.
	Source types.URLRef `json:"source"`
	// Subject - The subject of the event in the context of the event producer
	// (identified by `source`).
	Subject *string `json:"subject,omitempty"`
	// ID of the event; must be non-empty and unique within the scope of the producer.
	ID string `json:"id"`
	// Time - A Timestamp when the event happened.
	Time *types.Timestamp `json:"time,omitempty"`
	// SchemaURL - A link to the schema that the `data` attribute adheres to.
	SchemaURL *types.URLRef `json:"schemaurl,omitempty"`
	// GetDataMediaType - A MIME (RFC2046) string describing the media type of `data`.
	// TODO: Should an empty string assume `application/json`, `application/octet-stream`, or auto-detect the content?
	DataContentType *string `json:"datacontenttype,omitempty"`
	// DataContentEncoding describes the content encoding for the `data` attribute. Valid: nil, `Base64`.
	DataContentEncoding *string `json:"datacontentencoding,omitempty"`
	// Extensions - Additional extension metadata beyond the base spec.
	Extensions map[string]interface{} `json:"-"`
}

// Adhere to EventContext
var _ EventContext = (*EventContextV03)(nil)

// ExtensionAs implements EventContext.ExtensionAs
func (ec EventContextV03) ExtensionAs(name string, obj interface{}) error {
	value, ok := ec.Extensions[name]
	if !ok {
		return fmt.Errorf("extension %q does not exist", name)
	}

	// Try to unmarshal extension if we find it as a RawMessage.
	switch v := value.(type) {
	case json.RawMessage:
		if err := json.Unmarshal(v, obj); err == nil {
			// if that worked, return with obj set.
			return nil
		}
	}
	// else try as a string ptr.

	// Only support *string for now.
	switch v := obj.(type) {
	case *string:
		if valueAsString, ok := value.(string); ok {
			*v = valueAsString
			return nil
		} else {
			return fmt.Errorf("invalid type for extension %q", name)
		}
	default:
		return fmt.Errorf("unkown extension type %T", obj)
	}
}

// SetExtension adds the extension 'name' with value 'value' to the CloudEvents context.
func (ec *EventContextV03) SetExtension(name string, value interface{}) error {
	if ec.Extensions == nil {
		ec.Extensions = make(map[string]interface{})
	}
	if value == nil {
		delete(ec.Extensions, name)
	} else {
		ec.Extensions[name] = value
	}
	return nil
}

// Clone implements EventContextConverter.Clone
func (ec EventContextV03) Clone() EventContext {
	return ec.AsV03()
}

// AsV01 implements EventContextConverter.AsV01
func (ec EventContextV03) AsV01() *EventContextV01 {
	ecv2 := ec.AsV02()
	return ecv2.AsV01()
}

// AsV02 implements EventContextConverter.AsV02
func (ec EventContextV03) AsV02() *EventContextV02 {
	ret := EventContextV02{
		SpecVersion: CloudEventsVersionV02,
		ID:          ec.ID,
		Time:        ec.Time,
		Type:        ec.Type,
		SchemaURL:   ec.SchemaURL,
		ContentType: ec.DataContentType,
		Source:      ec.Source,
		Extensions:  make(map[string]interface{}),
	}
	// Subject was introduced in 0.3, so put it in an extension for 0.2.
	if ec.Subject != nil {
		ret.SetExtension(SubjectKey, *ec.Subject)
	}
	// DataContentEncoding was introduced in 0.3, so put it in an extension for 0.2.
	if ec.DataContentEncoding != nil {
		ret.SetExtension(DataContentEncodingKey, *ec.DataContentEncoding)
	}
	if ec.Extensions != nil {
		for k, v := range ec.Extensions {
			ret.Extensions[k] = v
		}
	}
	if len(ret.Extensions) == 0 {
		ret.Extensions = nil
	}
	return &ret
}

// AsV03 implements EventContextConverter.AsV03
func (ec EventContextV03) AsV03() *EventContextV03 {
	ec.SpecVersion = CloudEventsVersionV03
	return &ec
}

// Validate returns errors based on requirements from the CloudEvents spec.
// For more details, see https://github.com/cloudevents/spec/blob/master/spec.md
// As of Feb 26, 2019, commit 17c32ea26baf7714ad027d9917d03d2fff79fc7e
// + https://github.com/cloudevents/spec/pull/387 -> datacontentencoding
// + https://github.com/cloudevents/spec/pull/406 -> subject
func (ec EventContextV03) Validate() error {
	errors := []string(nil)

	// type
	// Type: String
	// Constraints:
	//  REQUIRED
	//  MUST be a non-empty string
	//  SHOULD be prefixed with a reverse-DNS name. The prefixed domain dictates the organization which defines the semantics of this event type.
	eventType := strings.TrimSpace(ec.Type)
	if eventType == "" {
		errors = append(errors, "type: MUST be a non-empty string")
	}

	// specversion
	// Type: String
	// Constraints:
	//  REQUIRED
	//  MUST be a non-empty string
	specVersion := strings.TrimSpace(ec.SpecVersion)
	if specVersion == "" {
		errors = append(errors, "specversion: MUST be a non-empty string")
	}

	// source
	// Type: URI-reference
	// Constraints:
	//  REQUIRED
	source := strings.TrimSpace(ec.Source.String())
	if source == "" {
		errors = append(errors, "source: REQUIRED")
	}

	// subject
	// Type: String
	// Constraints:
	//  OPTIONAL
	//  MUST be a non-empty string
	if ec.Subject != nil {
		subject := strings.TrimSpace(*ec.Subject)
		if subject == "" {
			errors = append(errors, "subject: if present, MUST be a non-empty string")
		}
	}

	// id
	// Type: String
	// Constraints:
	//  REQUIRED
	//  MUST be a non-empty string
	//  MUST be unique within the scope of the producer
	id := strings.TrimSpace(ec.ID)
	if id == "" {
		errors = append(errors, "id: MUST be a non-empty string")

		// no way to test "MUST be unique within the scope of the producer"
	}

	// time
	// Type: Timestamp
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 3339
	// --> no need to test this, no way to set the time without it being valid.

	// schemaurl
	// Type: URI
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 3986
	if ec.SchemaURL != nil {
		schemaURL := strings.TrimSpace(ec.SchemaURL.String())
		// empty string is not RFC 3986 compatible.
		if schemaURL == "" {
			errors = append(errors, "schemaurl: if present, MUST adhere to the format specified in RFC 3986")
		}
	}

	// datacontenttype
	// Type: String per RFC 2046
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 2046
	if ec.DataContentType != nil {
		dataContentType := strings.TrimSpace(*ec.DataContentType)
		if dataContentType == "" {
			// TODO: need to test for RFC 2046
			errors = append(errors, "datacontenttype: if present, MUST adhere to the format specified in RFC 2046")
		}
	}

	// datacontentencoding
	// Type: String per RFC 2045 Section 6.1
	// Constraints:
	//  The attribute MUST be set if the data attribute contains string-encoded binary data.
	//    Otherwise the attribute MUST NOT be set.
	//  If present, MUST adhere to RFC 2045 Section 6.1
	if ec.DataContentEncoding != nil {
		dataContentEncoding := strings.ToLower(strings.TrimSpace(*ec.DataContentEncoding))
		if dataContentEncoding != Base64 {
			// TODO: need to test for RFC 2046
			errors = append(errors, "datacontentencoding: if present, MUST adhere to RFC 2045 Section 6.1")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}

// String returns a pretty-printed representation of the EventContext.
func (ec EventContextV03) String() string {
	b := strings.Builder{}

	b.WriteString("Context Attributes,\n")

	b.WriteString("  specversion: " + ec.SpecVersion + "\n")
	b.WriteString("  type: " + ec.Type + "\n")
	b.WriteString("  source: " + ec.Source.String() + "\n")
	if ec.Subject != nil {
		b.WriteString("  subject: " + *ec.Subject + "\n")
	}
	b.WriteString("  id: " + ec.ID + "\n")
	if ec.Time != nil {
		b.WriteString("  time: " + ec.Time.String() + "\n")
	}
	if ec.SchemaURL != nil {
		b.WriteString("  schemaurl: " + ec.SchemaURL.String() + "\n")
	}
	if ec.DataContentType != nil {
		b.WriteString("  datacontenttype: " + *ec.DataContentType + "\n")
	}
	if ec.DataContentEncoding != nil {
		b.WriteString("  datacontentencoding: " + *ec.DataContentEncoding + "\n")
	}

	if ec.Extensions != nil && len(ec.Extensions) > 0 {
		b.WriteString("Extensions,\n")
		keys := make([]string, 0, len(ec.Extensions))
		for k := range ec.Extensions {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(fmt.Sprintf("  %s: %v\n", key, ec.Extensions[key]))
		}
	}

	return b.String()
}
