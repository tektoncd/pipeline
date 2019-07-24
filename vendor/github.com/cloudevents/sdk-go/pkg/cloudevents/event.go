package cloudevents

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Event represents the canonical representation of a CloudEvent.
type Event struct {
	Context     EventContext
	Data        interface{}
	DataEncoded bool
}

const (
	defaultEventVersion = CloudEventsVersionV02
)

// New returns a new Event, an optional version can be passed to change the
// default spec version from 0.2 to the provided version.
func New(version ...string) Event {
	specVersion := defaultEventVersion // TODO: should there be a default? or set a default?
	if len(version) >= 1 {
		specVersion = version[0]
	}
	e := &Event{}
	e.SetSpecVersion(specVersion)
	return *e
}

// ExtensionAs returns Context.ExtensionAs(name, obj)
func (e Event) ExtensionAs(name string, obj interface{}) error {
	return e.Context.ExtensionAs(name, obj)
}

// Validate performs a spec based validation on this event.
// Validation is dependent on the spec version specified in the event context.
func (e Event) Validate() error {
	if e.Context == nil {
		return fmt.Errorf("every event conforming to the CloudEvents specification MUST include a context")
	}

	if err := e.Context.Validate(); err != nil {
		return err
	}

	// TODO: validate data.

	return nil
}

// String returns a pretty-printed representation of the Event.
func (e Event) String() string {
	b := strings.Builder{}

	b.WriteString("Validation: ")

	valid := e.Validate()
	if valid == nil {
		b.WriteString("valid\n")
	} else {
		b.WriteString("invalid\n")
	}
	if valid != nil {
		b.WriteString(fmt.Sprintf("Validation Error: \n%s\n", valid.Error()))
	}

	b.WriteString(e.Context.String())

	if e.Data != nil {
		b.WriteString("Data,\n  ")
		if strings.HasPrefix(e.DataContentType(), ApplicationJSON) {
			var prettyJSON bytes.Buffer

			data, ok := e.Data.([]byte)
			if !ok {
				var err error
				data, err = json.Marshal(e.Data)
				if err != nil {
					data = []byte(err.Error())
				}
			}
			err := json.Indent(&prettyJSON, data, "  ", "  ")
			if err != nil {
				b.Write(e.Data.([]byte))
			} else {
				b.Write(prettyJSON.Bytes())
			}
		} else {
			b.Write(e.Data.([]byte))
		}
		b.WriteString("\n")
	}
	return b.String()
}
