package cloudevents

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
)

// Adhere to EventContextWriter
var _ EventContextWriter = (*EventContextV02)(nil)

// SetSpecVersion implements EventContextWriter.SetSpecVersion
func (ec *EventContextV02) SetSpecVersion(v string) error {
	if v != CloudEventsVersionV02 {
		return fmt.Errorf("invalid version %q, expecting %q", v, CloudEventsVersionV02)
	}
	ec.SpecVersion = CloudEventsVersionV02
	return nil
}

// SetDataContentType implements EventContextWriter.SetDataContentType
func (ec *EventContextV02) SetDataContentType(ct string) error {
	ct = strings.TrimSpace(ct)
	if ct == "" {
		ec.ContentType = nil
	} else {
		ec.ContentType = &ct
	}
	return nil
}

// SetType implements EventContextWriter.SetType
func (ec *EventContextV02) SetType(t string) error {
	t = strings.TrimSpace(t)
	ec.Type = t
	return nil
}

// SetSource implements EventContextWriter.SetSource
func (ec *EventContextV02) SetSource(u string) error {
	pu, err := url.Parse(u)
	if err != nil {
		return err
	}
	ec.Source = types.URLRef{URL: *pu}
	return nil
}

// SetSubject implements EventContextWriter.SetSubject
func (ec *EventContextV02) SetSubject(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return ec.SetExtension(SubjectKey, nil)
	}
	return ec.SetExtension(SubjectKey, s)
}

// SetID implements EventContextWriter.SetID
func (ec *EventContextV02) SetID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("id is required to be a non-empty string")
	}
	ec.ID = id
	return nil
}

// SetTime implements EventContextWriter.SetTime
func (ec *EventContextV02) SetTime(t time.Time) error {
	if t.IsZero() {
		ec.Time = nil
	} else {
		ec.Time = &types.Timestamp{Time: t}
	}
	return nil
}

// SetDataSchema implements EventContextWriter.SetDataSchema
func (ec *EventContextV02) SetDataSchema(u string) error {
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

// DeprecatedSetDataContentEncoding implements EventContextWriter.DeprecatedSetDataContentEncoding
func (ec *EventContextV02) DeprecatedSetDataContentEncoding(e string) error {
	e = strings.ToLower(strings.TrimSpace(e))
	if e == "" {
		return ec.SetExtension(DataContentEncodingKey, nil)
	}
	return ec.SetExtension(DataContentEncodingKey, e)
}
