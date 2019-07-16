package cloudevents

import (
	"fmt"
	"time"
)

var _ EventWriter = (*Event)(nil)

// SetSpecVersion implements EventWriter.SetSpecVersion
func (e *Event) SetSpecVersion(v string) {
	if e.Context == nil {
		switch v {
		case CloudEventsVersionV01:
			e.Context = EventContextV01{}.AsV01()
		case CloudEventsVersionV02:
			e.Context = EventContextV02{}.AsV02()
		case CloudEventsVersionV03:
			e.Context = EventContextV03{}.AsV03()
		default:
			panic(fmt.Errorf("a valid spec version is required: [%s, %s, %s]",
				CloudEventsVersionV01, CloudEventsVersionV02, CloudEventsVersionV03))
		}
		return
	}
	if err := e.Context.SetSpecVersion(v); err != nil {
		panic(err)
	}
}

// SetType implements EventWriter.SetType
func (e *Event) SetType(t string) {
	if err := e.Context.SetType(t); err != nil {
		panic(err)
	}
}

// SetSource implements EventWriter.SetSource
func (e *Event) SetSource(s string) {
	if err := e.Context.SetSource(s); err != nil {
		panic(err)
	}
}

// SetSubject implements EventWriter.SetSubject
func (e *Event) SetSubject(s string) {
	if err := e.Context.SetSubject(s); err != nil {
		panic(err)
	}
}

// SetID implements EventWriter.SetID
func (e *Event) SetID(id string) {
	if err := e.Context.SetID(id); err != nil {
		panic(err)
	}
}

// SetTime implements EventWriter.SetTime
func (e *Event) SetTime(t time.Time) {
	if err := e.Context.SetTime(t); err != nil {
		panic(err)
	}
}

// SetSchemaURL implements EventWriter.SetSchemaURL
func (e *Event) SetSchemaURL(s string) {
	if err := e.Context.SetSchemaURL(s); err != nil {
		panic(err)
	}
}

// SetDataContentType implements EventWriter.SetDataContentType
func (e *Event) SetDataContentType(ct string) {
	if err := e.Context.SetDataContentType(ct); err != nil {
		panic(err)
	}
}

// SetDataContentEncoding implements EventWriter.SetDataContentEncoding
func (e *Event) SetDataContentEncoding(enc string) {
	if err := e.Context.SetDataContentEncoding(enc); err != nil {
		panic(err)
	}
}

// SetDataContentEncoding implements EventWriter.SetDataContentEncoding
func (e *Event) SetExtension(name string, obj interface{}) {
	if err := e.Context.SetExtension(name, obj); err != nil {
		panic(err)
	}
}
