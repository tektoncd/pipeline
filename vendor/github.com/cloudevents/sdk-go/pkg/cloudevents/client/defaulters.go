package client

import (
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/google/uuid"
	"time"
)

// EventDefaulter is the function signature for extensions that are able
// to perform event defaulting.
type EventDefaulter func(event cloudevents.Event) cloudevents.Event

// DefaultIDToUUIDIfNotSet will inspect the provided event and assign a UUID to
// context.ID if it is found to be empty.
func DefaultIDToUUIDIfNotSet(event cloudevents.Event) cloudevents.Event {
	if event.Context != nil {
		if event.ID() == "" {
			event.Context = event.Context.Clone()
			event.SetID(uuid.New().String())
		}
	}
	return event
}

// DefaultTimeToNowIfNotSet will inspect the provided event and assign a new
// Timestamp to context.Time if it is found to be nil or zero.
func DefaultTimeToNowIfNotSet(event cloudevents.Event) cloudevents.Event {
	if event.Context != nil {
		if event.Time().IsZero() {
			event.Context = event.Context.Clone()
			event.SetTime(time.Now())
		}
	}
	return event
}
