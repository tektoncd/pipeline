package cloudevents

import (
	"mime"
	"time"
)

// Adhere to EventContextReader
var _ EventContextReader = (*EventContextV01)(nil)

// GetSpecVersion implements EventContextReader.GetSpecVersion
func (ec EventContextV01) GetSpecVersion() string {
	if ec.CloudEventsVersion != "" {
		return ec.CloudEventsVersion
	}
	return CloudEventsVersionV01
}

// GetDataContentType implements EventContextReader.GetDataContentType
func (ec EventContextV01) GetDataContentType() string {
	if ec.ContentType != nil {
		return *ec.ContentType
	}
	return ""
}

// GetDataMediaType implements EventContextReader.GetDataMediaType
func (ec EventContextV01) GetDataMediaType() (string, error) {
	if ec.ContentType != nil {
		mediaType, _, err := mime.ParseMediaType(*ec.ContentType)
		if err != nil {
			return "", err
		}
		return mediaType, nil
	}
	return "", nil
}

// GetType implements EventContextReader.GetType
func (ec EventContextV01) GetType() string {
	return ec.EventType
}

// GetSource implements EventContextReader.GetSource
func (ec EventContextV01) GetSource() string {
	return ec.Source.String()
}

// GetSubject implements EventContextReader.GetSubject
func (ec EventContextV01) GetSubject() string {
	var sub string
	if err := ec.ExtensionAs(SubjectKey, &sub); err != nil {
		return ""
	}
	return sub
}

// GetID implements EventContextReader.GetID
func (ec EventContextV01) GetID() string {
	return ec.EventID
}

// GetTime implements EventContextReader.GetTime
func (ec EventContextV01) GetTime() time.Time {
	if ec.EventTime != nil {
		return ec.EventTime.Time
	}
	return time.Time{}
}

// GetSchemaURL implements EventContextReader.GetSchemaURL
func (ec EventContextV01) GetSchemaURL() string {
	if ec.SchemaURL != nil {
		return ec.SchemaURL.String()
	}
	return ""
}

// GetDataContentEncoding implements EventContextReader.GetDataContentEncoding
func (ec EventContextV01) GetDataContentEncoding() string {
	var enc string
	if err := ec.ExtensionAs(DataContentEncodingKey, &enc); err != nil {
		return ""
	}
	return enc
}

func (ec EventContextV01) GetExtensions() map[string]interface{} {
	return ec.Extensions
}
