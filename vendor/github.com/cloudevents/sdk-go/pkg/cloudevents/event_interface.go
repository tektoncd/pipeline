package cloudevents

import (
	"time"
)

// EventWriter is the interface for reading through an event from attributes.
type EventReader interface {
	// SpecVersion returns event.Context.GetSpecVersion().
	SpecVersion() string
	// Type returns event.Context.GetType().
	Type() string
	// Source returns event.Context.GetSource().
	Source() string
	// Subject returns event.Context.GetSubject().
	Subject() string
	// ID returns event.Context.GetID().
	ID() string
	// Time returns event.Context.GetTime().
	Time() time.Time
	// SchemaURL returns event.Context.GetSchemaURL().
	SchemaURL() string
	// DataContentType returns event.Context.GetDataContentType().
	DataContentType() string
	// DataMediaType returns event.Context.GetDataMediaType().
	DataMediaType() string
	// DataContentEncoding returns event.Context.GetDataContentEncoding().
	DataContentEncoding() string

	// Extension Attributes

	// Extensions returns the event.Context.GetExtensions().
	Extensions() map[string]interface{}

	// ExtensionAs returns event.Context.ExtensionAs(name, obj).
	ExtensionAs(string, interface{}) error

	// Data Attribute

	// ExtensionAs returns event.Context.ExtensionAs(name, obj).
	DataAs(interface{}) error
}

// EventWriter is the interface for writing through an event onto attributes.
// If an error is thrown by a sub-component, EventWriter panics.
type EventWriter interface {
	// Context Attributes

	// SetSpecVersion performs event.Context.SetSpecVersion.
	SetSpecVersion(string)
	// SetType performs event.Context.SetType.
	SetType(string)
	// SetSource performs event.Context.SetSource.
	SetSource(string)
	// SetSubject( performs event.Context.SetSubject.
	SetSubject(string)
	// SetID performs event.Context.SetID.
	SetID(string)
	// SetTime performs event.Context.SetTime.
	SetTime(time.Time)
	// SetSchemaURL performs event.Context.SetSchemaURL.
	SetSchemaURL(string)
	// SetDataContentType performs event.Context.SetDataContentType.
	SetDataContentType(string)
	// SetDataContentEncoding performs event.Context.SetDataContentEncoding.
	SetDataContentEncoding(string)

	// Extension Attributes

	// SetExtension performs event.Context.SetExtension.
	SetExtension(string, interface{})

	// SetData encodes the given payload with the current encoding settings.
	SetData(interface{}) error
}
