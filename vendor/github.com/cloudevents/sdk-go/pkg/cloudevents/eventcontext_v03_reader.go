package cloudevents

import (
	"mime"
	"time"
)

// GetSpecVersion implements EventContextReader.GetSpecVersion
func (ec EventContextV03) GetSpecVersion() string {
	if ec.SpecVersion != "" {
		return ec.SpecVersion
	}
	return CloudEventsVersionV03
}

// GetDataContentType implements EventContextReader.GetDataContentType
func (ec EventContextV03) GetDataContentType() string {
	if ec.DataContentType != nil {
		return *ec.DataContentType
	}
	return ""
}

// GetDataMediaType implements EventContextReader.GetDataMediaType
func (ec EventContextV03) GetDataMediaType() (string, error) {
	if ec.DataContentType != nil {
		mediaType, _, err := mime.ParseMediaType(*ec.DataContentType)
		if err != nil {
			return "", err
		}
		return mediaType, nil
	}
	return "", nil
}

// GetType implements EventContextReader.GetType
func (ec EventContextV03) GetType() string {
	return ec.Type
}

// GetSource implements EventContextReader.GetSource
func (ec EventContextV03) GetSource() string {
	return ec.Source.String()
}

// GetSubject implements EventContextReader.GetSubject
func (ec EventContextV03) GetSubject() string {
	if ec.Subject != nil {
		return *ec.Subject
	}
	return ""
}

// GetTime implements EventContextReader.GetTime
func (ec EventContextV03) GetTime() time.Time {
	if ec.Time != nil {
		return ec.Time.Time
	}
	return time.Time{}
}

// GetID implements EventContextReader.GetID
func (ec EventContextV03) GetID() string {
	return ec.ID
}

// GetSchemaURL implements EventContextReader.GetSchemaURL
func (ec EventContextV03) GetSchemaURL() string {
	if ec.SchemaURL != nil {
		return ec.SchemaURL.String()
	}
	return ""
}

// GetDataContentEncoding implements EventContextReader.GetDataContentEncoding
func (ec EventContextV03) GetDataContentEncoding() string {
	if ec.DataContentEncoding != nil {
		return *ec.DataContentEncoding
	}
	return ""
}

func (ec EventContextV03) GetExtensions() map[string]interface{} {
	return ec.Extensions
}
