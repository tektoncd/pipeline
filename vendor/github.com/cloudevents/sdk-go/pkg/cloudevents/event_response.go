package cloudevents

// EventResponse represents the canonical representation of a Response to a
// CloudEvent from a receiver. Response implementation is Transport dependent.
type EventResponse struct {
	Status int
	Event  *Event
	Reason string
	// Context is transport specific struct to allow for controlling transport
	// response details.
	// For example, see http.TransportResponseContext.
	Context interface{}
}

// RespondWith sets up the instance of EventResponse to be set with status and
// an event. Response implementation is Transport dependent.
func (e *EventResponse) RespondWith(status int, event *Event) {
	if e == nil {
		// if nil, response not supported
		return
	}
	e.Status = status
	if event != nil {
		e.Event = event
	}
}

// Error sets the instance of EventResponse to be set with an error code and
// reason string. Response implementation is Transport dependent.
func (e *EventResponse) Error(status int, reason string) {
	if e == nil {
		// if nil, response not supported
		return
	}
	e.Status = status
	e.Reason = reason
}
