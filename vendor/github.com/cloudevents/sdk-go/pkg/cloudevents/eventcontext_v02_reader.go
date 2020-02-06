package cloudevents

import (
	"fmt"
	"mime"
	"time"
)

// Adhere to EventContextReader
var _ EventContextReader = (*EventContextV02)(nil)

// GetSpecVersion implements EventContextReader.GetSpecVersion
func (ec EventContextV02) GetSpecVersion() string {
	if ec.SpecVersion != "" {
		return ec.SpecVersion
	}
	return CloudEventsVersionV02
}

// GetType implements EventContextReader.GetType
func (ec EventContextV02) GetType() string {
	return ec.Type
}

// GetSource implements EventContextReader.GetSource
func (ec EventContextV02) GetSource() string {
	return ec.Source.String()
}

// GetSubject implements EventContextReader.GetSubject
func (ec EventContextV02) GetSubject() string {
	var sub string
	if err := ec.ExtensionAs(SubjectKey, &sub); err != nil {
		return ""
	}
	return sub
}

// GetID implements EventContextReader.GetID
func (ec EventContextV02) GetID() string {
	return ec.ID
}

// GetTime implements EventContextReader.GetTime
func (ec EventContextV02) GetTime() time.Time {
	if ec.Time != nil {
		return ec.Time.Time
	}
	return time.Time{}
}

// GetDataSchema implements EventContextReader.GetDataSchema
func (ec EventContextV02) GetDataSchema() string {
	if ec.SchemaURL != nil {
		return ec.SchemaURL.String()
	}
	return ""
}

// GetDataContentType implements EventContextReader.GetDataContentType
func (ec EventContextV02) GetDataContentType() string {
	if ec.ContentType != nil {
		return *ec.ContentType
	}
	return ""
}

// GetDataMediaType implements EventContextReader.GetDataMediaType
func (ec EventContextV02) GetDataMediaType() (string, error) {
	if ec.ContentType != nil {
		mediaType, _, err := mime.ParseMediaType(*ec.ContentType)
		if err != nil {
			return "", err
		}
		return mediaType, nil
	}
	return "", nil
}

// DeprecatedGetDataContentEncoding implements EventContextReader.DeprecatedGetDataContentEncoding
func (ec EventContextV02) DeprecatedGetDataContentEncoding() string {
	var enc string
	if err := ec.ExtensionAs(DataContentEncodingKey, &enc); err != nil {
		return ""
	}
	return enc
}

// GetExtensions implements EventContextReader.GetExtensions
func (ec EventContextV02) GetExtensions() map[string]interface{} {
	return ec.Extensions
}

// GetExtension implements EventContextReader.GetExtension
func (ec EventContextV02) GetExtension(key string) (interface{}, error) {
	v, ok := caseInsensitiveSearch(key, ec.Extensions)
	if !ok {
		return "", fmt.Errorf("%q not found", key)
	}
	return v, nil
}
