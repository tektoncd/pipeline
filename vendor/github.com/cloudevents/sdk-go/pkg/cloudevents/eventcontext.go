package cloudevents

import "time"

// EventContextReader are the methods required to be a reader of context
// attributes.
type EventContextReader interface {
	// GetSpecVersion returns the native CloudEvents Spec version of the event
	// context.
	GetSpecVersion() string
	// GetType returns the CloudEvents type from the context.
	GetType() string
	// GetSource returns the CloudEvents source from the context.
	GetSource() string
	// GetSubject returns the CloudEvents subject from the context.
	GetSubject() string
	// GetID returns the CloudEvents ID from the context.
	GetID() string
	// GetTime returns the CloudEvents creation time from the context.
	GetTime() time.Time
	// GetSchemaURL returns the CloudEvents schema URL (if any) from the
	// context.
	GetSchemaURL() string
	// GetDataContentType returns content type on the context.
	GetDataContentType() string
	// GetDataContentEncoding returns content encoding on the context.
	GetDataContentEncoding() string

	// GetDataMediaType returns the MIME media type for encoded data, which is
	// needed by both encoding and decoding. This is a processed form of
	// GetDataContentType and it may return an error.
	GetDataMediaType() (string, error)

	// ExtensionAs populates the given interface with the CloudEvents extension
	// of the given name from the extension attributes. It returns an error if
	// the extension does not exist, the extension's type does not match the
	// provided type, or if the type is not a supported.
	ExtensionAs(string, interface{}) error

	// GetExtensions returns the full extensions map.
	GetExtensions() map[string]interface{}
}

// EventContextWriter are the methods required to be a writer of context
// attributes.
type EventContextWriter interface {
	// SetSpecVersion sets the spec version of the context.
	SetSpecVersion(string) error
	// SetType sets the type of the context.
	SetType(string) error
	// SetSource sets the source of the context.
	SetSource(string) error
	// SetSubject sets the subject of the context.
	SetSubject(string) error
	// SetID sets the ID of the context.
	SetID(string) error
	// SetTime sets the time of the context.
	SetTime(time time.Time) error
	// SetSchemaURL sets the schema url of the context.
	SetSchemaURL(string) error
	// SetDataContentType sets the data content type of the context.
	SetDataContentType(string) error
	// SetDataContentEncoding sets the data context encoding of the context.
	SetDataContentEncoding(string) error

	// SetExtension sets the given interface onto the extension attributes
	// determined by the provided name.
	SetExtension(string, interface{}) error
}

type EventContextConverter interface {
	// AsV01 provides a translation from whatever the "native" encoding of the
	// CloudEvent was to the equivalent in v0.1 field names, moving fields to or
	// from extensions as necessary.
	AsV01() *EventContextV01

	// AsV02 provides a translation from whatever the "native" encoding of the
	// CloudEvent was to the equivalent in v0.2 field names, moving fields to or
	// from extensions as necessary.
	AsV02() *EventContextV02

	// AsV03 provides a translation from whatever the "native" encoding of the
	// CloudEvent was to the equivalent in v0.3 field names, moving fields to or
	// from extensions as necessary.
	AsV03() *EventContextV03
}

// EventContext is conical interface for a CloudEvents Context.
type EventContext interface {
	// EventContextConverter allows for conversion between versions.
	EventContextConverter

	// EventContextReader adds methods for reading context.
	EventContextReader

	// EventContextWriter adds methods for writing to context.
	EventContextWriter

	// Validate the event based on the specifics of the CloudEvents spec version
	// represented by this event context.
	Validate() error

	// Clone clones the event context.
	Clone() EventContext

	// String returns a pretty-printed representation of the EventContext.
	String() string
}
