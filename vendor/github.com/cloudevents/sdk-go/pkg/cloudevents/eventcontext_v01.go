package cloudevents

import (
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"sort"
	"strings"
)

const (
	// CloudEventsVersionV01 represents the version 0.1 of the CloudEvents spec.
	CloudEventsVersionV01 = "0.1"
)

// EventContextV01 holds standard metadata about an event. See
// https://github.com/cloudevents/spec/blob/v0.1/spec.md#context-attributes for
// details on these fields.
type EventContextV01 struct {
	// The version of the CloudEvents specification used by the event.
	CloudEventsVersion string `json:"cloudEventsVersion,omitempty"`
	// ID of the event; must be non-empty and unique within the scope of the producer.
	EventID string `json:"eventID"`
	// Timestamp when the event happened.
	EventTime *types.Timestamp `json:"eventTime,omitempty"`
	// Type of occurrence which has happened.
	EventType string `json:"eventType"`
	// The version of the `eventType`; this is producer-specific.
	EventTypeVersion *string `json:"eventTypeVersion,omitempty"`
	// A link to the schema that the `data` attribute adheres to.
	SchemaURL *types.URLRef `json:"schemaURL,omitempty"`
	// A MIME (RFC 2046) string describing the media type of `data`.
	// TODO: Should an empty string assume `application/json`, or auto-detect the content?
	ContentType *string `json:"contentType,omitempty"`
	// A URI describing the event producer.
	Source types.URLRef `json:"source"`
	// Additional metadata without a well-defined structure.
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

// Adhere to EventContext
var _ EventContext = (*EventContextV01)(nil)

// ExtensionAs implements EventContextReader.ExtensionAs
func (ec EventContextV01) ExtensionAs(name string, obj interface{}) error {
	value, ok := ec.Extensions[name]
	if !ok {
		return fmt.Errorf("extension %q does not exist", name)
	}
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
func (ec *EventContextV01) SetExtension(name string, value interface{}) error {
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
func (ec EventContextV01) Clone() EventContext {
	return ec.AsV01()
}

// AsV01 implements EventContextConverter.AsV01
func (ec EventContextV01) AsV01() *EventContextV01 {
	ec.CloudEventsVersion = CloudEventsVersionV01
	return &ec
}

// AsV02 implements EventContextConverter.AsV02
func (ec EventContextV01) AsV02() *EventContextV02 {
	ret := EventContextV02{
		SpecVersion: CloudEventsVersionV02,
		Type:        ec.EventType,
		Source:      ec.Source,
		ID:          ec.EventID,
		Time:        ec.EventTime,
		SchemaURL:   ec.SchemaURL,
		ContentType: ec.ContentType,
		Extensions:  make(map[string]interface{}),
	}

	// eventTypeVersion was retired in v0.2, so put it in an extension.
	if ec.EventTypeVersion != nil {
		ret.SetExtension(EventTypeVersionKey, *ec.EventTypeVersion)
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
func (ec EventContextV01) AsV03() *EventContextV03 {
	ecv2 := ec.AsV02()
	return ecv2.AsV03()
}

// Validate returns errors based on requirements from the CloudEvents spec.
// For more details, see https://github.com/cloudevents/spec/blob/v0.1/spec.md
func (ec EventContextV01) Validate() error {
	errors := []string(nil)

	// eventType
	// Type: String
	// Constraints:
	// 	REQUIRED
	// 	MUST be a non-empty string
	// 	SHOULD be prefixed with a reverse-DNS name. The prefixed domain dictates the organization which defines the semantics of this event type.
	eventType := strings.TrimSpace(ec.EventType)
	if eventType == "" {
		errors = append(errors, "eventType: MUST be a non-empty string")
	}

	// eventTypeVersion
	// Type: String
	// Constraints:
	// 	OPTIONAL
	// 	If present, MUST be a non-empty string
	if ec.EventTypeVersion != nil {
		eventTypeVersion := strings.TrimSpace(*ec.EventTypeVersion)
		if eventTypeVersion == "" {
			errors = append(errors, "eventTypeVersion: if present, MUST be a non-empty string")
		}
	}

	// cloudEventsVersion
	// Type: String
	// Constraints:
	// 	REQUIRED
	// 	MUST be a non-empty string
	cloudEventsVersion := strings.TrimSpace(ec.CloudEventsVersion)
	if cloudEventsVersion == "" {
		errors = append(errors, "cloudEventsVersion: MUST be a non-empty string")
	}

	// source
	// Type: URI
	// Constraints:
	// 	REQUIRED
	source := strings.TrimSpace(ec.Source.String())
	if source == "" {
		errors = append(errors, "source: REQUIRED")
	}

	// eventID
	// Type: String
	// Constraints:
	// 	REQUIRED
	// 	MUST be a non-empty string
	// 	MUST be unique within the scope of the producer
	eventID := strings.TrimSpace(ec.EventID)
	if eventID == "" {
		errors = append(errors, "eventID: MUST be a non-empty string")

		// no way to test "MUST be unique within the scope of the producer"
	}

	// eventTime
	// Type: Timestamp
	// Constraints:
	// 	OPTIONAL
	//	If present, MUST adhere to the format specified in RFC 3339
	// --> no need to test this, no way to set the eventTime without it being valid.

	// schemaURL
	// Type: URI
	// Constraints:
	// 	OPTIONAL
	// 	If present, MUST adhere to the format specified in RFC 3986
	if ec.SchemaURL != nil {
		schemaURL := strings.TrimSpace(ec.SchemaURL.String())
		// empty string is not RFC 3986 compatible.
		if schemaURL == "" {
			errors = append(errors, "schemaURL: if present, MUST adhere to the format specified in RFC 3986")
		}
	}

	// contentType
	// Type: String per RFC 2046
	// Constraints:
	// 	OPTIONAL
	// 	If present, MUST adhere to the format specified in RFC 2046
	if ec.ContentType != nil {
		contentType := strings.TrimSpace(*ec.ContentType)
		if contentType == "" {
			// TODO: need to test for RFC 2046
			errors = append(errors, "contentType: if present, MUST adhere to the format specified in RFC 2046")
		}
	}

	// extensions
	// Type: Map
	// Constraints:
	// 	OPTIONAL
	// 	If present, MUST contain at least one entry
	if ec.Extensions != nil {
		if len(ec.Extensions) == 0 {
			errors = append(errors, "extensions: if present, MUST contain at least one entry")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf(strings.Join(errors, "\n"))
	}
	return nil
}

// String returns a pretty-printed representation of the EventContext.
func (ec EventContextV01) String() string {
	b := strings.Builder{}

	b.WriteString("Context Attributes,\n")

	b.WriteString("  cloudEventsVersion: " + ec.CloudEventsVersion + "\n")
	b.WriteString("  eventType: " + ec.EventType + "\n")
	if ec.EventTypeVersion != nil {
		b.WriteString("  eventTypeVersion: " + *ec.EventTypeVersion + "\n")
	}
	b.WriteString("  source: " + ec.Source.String() + "\n")
	b.WriteString("  eventID: " + ec.EventID + "\n")
	if ec.EventTime != nil {
		b.WriteString("  eventTime: " + ec.EventTime.String() + "\n")
	}
	if ec.SchemaURL != nil {
		b.WriteString("  schemaURL: " + ec.SchemaURL.String() + "\n")
	}
	if ec.ContentType != nil {
		b.WriteString("  contentType: " + *ec.ContentType + "\n")
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
