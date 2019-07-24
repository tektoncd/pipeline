package cloudevents

import (
	"time"
)

var _ EventReader = (*Event)(nil)

// SpecVersion implements EventReader.SpecVersion
func (e Event) SpecVersion() string {
	return e.Context.GetSpecVersion()
}

// Type implements EventReader.Type
func (e Event) Type() string {
	return e.Context.GetType()
}

// Source implements EventReader.Source
func (e Event) Source() string {
	return e.Context.GetSource()
}

// Subject implements EventReader.Subject
func (e Event) Subject() string {
	return e.Context.GetSubject()
}

// ID implements EventReader.ID
func (e Event) ID() string {
	return e.Context.GetID()
}

// Time implements EventReader.Time
func (e Event) Time() time.Time {
	return e.Context.GetTime()
}

// SchemaURL implements EventReader.SchemaURL
func (e Event) SchemaURL() string {
	return e.Context.GetSchemaURL()
}

// DataContentType implements EventReader.DataContentType
func (e Event) DataContentType() string {
	return e.Context.GetDataContentType()
}

// DataMediaType returns the parsed DataMediaType of the event. If parsing
// fails, the empty string is returned. To retrieve the parsing error, use
// `Context.GetDataMediaType` instead.
func (e Event) DataMediaType() string {
	mediaType, _ := e.Context.GetDataMediaType()
	return mediaType
}

// DataContentEncoding implements EventReader.DataContentEncoding
func (e Event) DataContentEncoding() string {
	return e.Context.GetDataContentEncoding()
}

// DataContentEncoding implements EventReader.DataContentEncoding
func (e Event) Extensions() map[string]interface{} {
	return e.Context.GetExtensions()
}
