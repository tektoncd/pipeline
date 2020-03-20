package event

import (
	"fmt"
	"mime"
	"time"
)

// GetSpecVersion implements EventContextReader.GetSpecVersion
func (ec EventContextV03) GetSpecVersion() string {
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

// GetDataSchema implements EventContextReader.GetDataSchema
func (ec EventContextV03) GetDataSchema() string {
	if ec.SchemaURL != nil {
		return ec.SchemaURL.String()
	}
	return ""
}

// DeprecatedGetDataContentEncoding implements EventContextReader.DeprecatedGetDataContentEncoding
func (ec EventContextV03) DeprecatedGetDataContentEncoding() string {
	if ec.DataContentEncoding != nil {
		return *ec.DataContentEncoding
	}
	return ""
}

// GetExtensions implements EventContextReader.GetExtensions
func (ec EventContextV03) GetExtensions() map[string]interface{} {
	return ec.Extensions
}

// GetExtension implements EventContextReader.GetExtension
func (ec EventContextV03) GetExtension(key string) (interface{}, error) {
	v, ok := caseInsensitiveSearch(key, ec.Extensions)
	if !ok {
		return "", fmt.Errorf("%q not found", key)
	}
	return v, nil
}
