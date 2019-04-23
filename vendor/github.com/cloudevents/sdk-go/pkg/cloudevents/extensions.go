package cloudevents

const (
	// DataContentEncodingKey is the key to DataContentEncoding for versions that do not support data content encoding
	// directly.
	DataContentEncodingKey = "datacontentencoding"

	// EventTypeVersionKey is the key to EventTypeVersion for versions that do not support event type version directly.
	EventTypeVersionKey = "eventTypeVersion"

	// SubjectKey is the key to Subject for versions that do not support subject directly.
	SubjectKey = "subject"
)
