package cloudevents

import (
	"encoding/json"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"sort"
	"strings"
)

const (
	// CloudEventsVersionV02 represents the version 0.2 of the CloudEvents spec.
	CloudEventsVersionV02 = "0.2"
)

// EventContextV02 represents the non-data attributes of a CloudEvents v0.2
// event.
type EventContextV02 struct {
	// The version of the CloudEvents specification used by the event.
	SpecVersion string `json:"specversion"`
	// The type of the occurrence which has happened.
	Type string `json:"type"`
	// A URI describing the event producer.
	Source types.URLRef `json:"source"`
	// ID of the event; must be non-empty and unique within the scope of the producer.
	ID string `json:"id"`
	// Timestamp when the event happened.
	Time *types.Timestamp `json:"time,omitempty"`
	// A link to the schema that the `data` attribute adheres to.
	SchemaURL *types.URLRef `json:"schemaurl,omitempty"`
	// A MIME (RFC2046) string describing the media type of `data`.
	// TODO: Should an empty string assume `application/json`, `application/octet-stream`, or auto-detect the content?
	ContentType *string `json:"contenttype,omitempty"`
	// Additional extension metadata beyond the base spec.
	Extensions map[string]interface{} `json:"-"`
}

// Adhere to EventContext
var _ EventContext = (*EventContextV02)(nil)

// ExtensionAs implements EventContext.ExtensionAs
func (ec EventContextV02) ExtensionAs(name string, obj interface{}) error {
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
func (ec *EventContextV02) SetExtension(name string, value interface{}) error {
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
func (ec EventContextV02) Clone() EventContext {
	return ec.AsV02()
}

// AsV01 implements EventContextConverter.AsV01
func (ec EventContextV02) AsV01() *EventContextV01 {
	ret := EventContextV01{
		CloudEventsVersion: CloudEventsVersionV01,
		EventID:            ec.ID,
		EventTime:          ec.Time,
		EventType:          ec.Type,
		SchemaURL:          ec.SchemaURL,
		Source:             ec.Source,
		ContentType:        ec.ContentType,
		Extensions:         make(map[string]interface{}),
	}

	for k, v := range ec.Extensions {
		// eventTypeVersion was retired in v0.2
		if strings.EqualFold(k, EventTypeVersionKey) {
			etv, ok := v.(string)
			if ok && etv != "" {
				ret.EventTypeVersion = &etv
			}
			continue
		}
		ret.Extensions[k] = v
	}
	if len(ret.Extensions) == 0 {
		ret.Extensions = nil
	}
	return &ret
}

// AsV02 implements EventContextConverter.AsV02
func (ec EventContextV02) AsV02() *EventContextV02 {
	ec.SpecVersion = CloudEventsVersionV02
	return &ec
}

// AsV03 implements EventContextConverter.AsV03
func (ec EventContextV02) AsV03() *EventContextV03 {
	ret := EventContextV03{
		SpecVersion:     CloudEventsVersionV03,
		ID:              ec.ID,
		Time:            ec.Time,
		Type:            ec.Type,
		SchemaURL:       ec.SchemaURL,
		DataContentType: ec.ContentType,
		Source:          ec.Source,
		Extensions:      make(map[string]interface{}),
	}

	for k, v := range ec.Extensions {
		// Subject was introduced in 0.3
		if strings.EqualFold(k, SubjectKey) {
			sub, ok := v.(string)
			if ok && sub != "" {
				ret.Subject = &sub
			}
			continue
		}
		// DataContentEncoding was introduced in 0.3
		if strings.EqualFold(k, DataContentEncodingKey) {
			etv, ok := v.(string)
			if ok && etv != "" {
				ret.DataContentEncoding = &etv
			}
			continue
		}
		ret.Extensions[k] = v
	}
	if len(ret.Extensions) == 0 {
		ret.Extensions = nil
	}

	return &ret
}

// Validate returns errors based on requirements from the CloudEvents spec.
// For more details, see https://github.com/cloudevents/spec/blob/v0.2/spec.md
func (ec EventContextV02) Validate() error {
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

	// contenttype
	// Type: String per RFC 2046
	// Constraints:
	//  OPTIONAL
	//  If present, MUST adhere to the format specified in RFC 2046
	if ec.ContentType != nil {
		contentType := strings.TrimSpace(*ec.ContentType)
		if contentType == "" {
			// TODO: need to test for RFC 2046
			errors = append(errors, "contenttype: if present, MUST adhere to the format specified in RFC 2046")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}

// String returns a pretty-printed representation of the EventContext.
func (ec EventContextV02) String() string {
	b := strings.Builder{}

	b.WriteString("Context Attributes,\n")

	b.WriteString("  specversion: " + ec.SpecVersion + "\n")
	b.WriteString("  type: " + ec.Type + "\n")
	b.WriteString("  source: " + ec.Source.String() + "\n")
	b.WriteString("  id: " + ec.ID + "\n")
	if ec.Time != nil {
		b.WriteString("  time: " + ec.Time.String() + "\n")
	}
	if ec.SchemaURL != nil {
		b.WriteString("  schemaurl: " + ec.SchemaURL.String() + "\n")
	}
	if ec.ContentType != nil {
		b.WriteString("  contenttype: " + *ec.ContentType + "\n")
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
