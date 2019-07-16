package cloudevents

import (
	"errors"
	"fmt"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"net/url"
	"strings"
	"time"
)

// Adhere to EventContextWriter
var _ EventContextWriter = (*EventContextV01)(nil)

// SetSpecVersion implements EventContextWriter.SetSpecVersion
func (ec *EventContextV01) SetSpecVersion(v string) error {
	if v != CloudEventsVersionV01 {
		return fmt.Errorf("invalid version %q, expecting %q", v, CloudEventsVersionV01)
	}
	ec.CloudEventsVersion = CloudEventsVersionV01
	return nil
}

// SetDataContentType implements EventContextWriter.SetDataContentType
func (ec *EventContextV01) SetDataContentType(ct string) error {
	ct = strings.TrimSpace(ct)
	if ct == "" {
		ec.ContentType = nil
	} else {
		ec.ContentType = &ct
	}
	return nil
}

// SetType implements EventContextWriter.SetType
func (ec *EventContextV01) SetType(t string) error {
	t = strings.TrimSpace(t)
	ec.EventType = t
	return nil
}

// SetSource implements EventContextWriter.SetSource
func (ec *EventContextV01) SetSource(u string) error {
	pu, err := url.Parse(u)
	if err != nil {
		return err
	}
	ec.Source = types.URLRef{URL: *pu}
	return nil
}

// SetSubject implements EventContextWriter.SetSubject
func (ec *EventContextV01) SetSubject(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return ec.SetExtension(SubjectKey, nil)
	}
	return ec.SetExtension(SubjectKey, s)
}

// SetID implements EventContextWriter.SetID
func (ec *EventContextV01) SetID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("event id is required to be a non-empty string")
	}
	ec.EventID = id
	return nil
}

// SetTime implements EventContextWriter.SetTime
func (ec *EventContextV01) SetTime(t time.Time) error {
	if t.IsZero() {
		ec.EventTime = nil
	} else {
		ec.EventTime = &types.Timestamp{Time: t}
	}
	return nil
}

// SetSchemaURL implements EventContextWriter.SetSchemaURL
func (ec *EventContextV01) SetSchemaURL(u string) error {
	u = strings.TrimSpace(u)
	if u == "" {
		ec.SchemaURL = nil
		return nil
	}
	pu, err := url.Parse(u)
	if err != nil {
		return err
	}
	ec.SchemaURL = &types.URLRef{URL: *pu}
	return nil
}

// SetDataContentEncoding implements EventContextWriter.SetDataContentEncoding
func (ec *EventContextV01) SetDataContentEncoding(e string) error {
	e = strings.ToLower(strings.TrimSpace(e))
	if e == "" {
		return ec.SetExtension(DataContentEncodingKey, nil)
	}
	return ec.SetExtension(DataContentEncodingKey, e)
}
