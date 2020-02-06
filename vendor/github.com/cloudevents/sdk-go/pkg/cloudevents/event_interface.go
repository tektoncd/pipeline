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
	// DataSchema returns event.Context.GetDataSchema().
	DataSchema() string
	// DataContentType returns event.Context.GetDataContentType().
	DataContentType() string
	// DataMediaType returns event.Context.GetDataMediaType().
	DataMediaType() string
	// DeprecatedDataContentEncoding returns event.Context.DeprecatedGetDataContentEncoding().
	DeprecatedDataContentEncoding() string

	// Extension Attributes

	// Extensions returns the event.Context.GetExtensions().
	// Extensions use the CloudEvents type system, details in package cloudevents/types.
	Extensions() map[string]interface{}

	// DEPRECATED: see event.Context.ExtensionAs
	// ExtensionAs returns event.Context.ExtensionAs(name, obj).
	ExtensionAs(string, interface{}) error

	// Data Attribute

	// DataAs attempts to populate the provided data object with the event payload.
	// data should be a pointer type.
	DataAs(interface{}) error
}

// EventWriter is the interface for writing through an event onto attributes.
// If an error is thrown by a sub-component, EventWriter caches the error
// internally and exposes errors with a call to event.Validate().
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
	// SetDataSchema performs event.Context.SetDataSchema.
	SetDataSchema(string)
	// SetDataContentType performs event.Context.SetDataContentType.
	SetDataContentType(string)
	// DeprecatedSetDataContentEncoding performs event.Context.DeprecatedSetDataContentEncoding.
	SetDataContentEncoding(string)

	// Extension Attributes

	// SetExtension performs event.Context.SetExtension.
	SetExtension(string, interface{})

	// SetData encodes the given payload with the current encoding settings.
	SetData(interface{}) error
}
