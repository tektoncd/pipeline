package ocagent

const (
	maxAnnotationEventsPerSpan = 32
	maxMessageEventsPerSpan    = 128
)

type SpanConfig struct {
	AnnotationEventsPerSpan int
	MessageEventsPerSpan    int
}

func (spanConfig SpanConfig) GetAnnotationEventsPerSpan() int {
	if spanConfig.AnnotationEventsPerSpan <= 0 {
		return maxAnnotationEventsPerSpan
	}
	return spanConfig.AnnotationEventsPerSpan
}

func (spanConfig SpanConfig) GetMessageEventsPerSpan() int {
	if spanConfig.MessageEventsPerSpan <= 0 {
		return maxMessageEventsPerSpan
	}
	return spanConfig.MessageEventsPerSpan
}
