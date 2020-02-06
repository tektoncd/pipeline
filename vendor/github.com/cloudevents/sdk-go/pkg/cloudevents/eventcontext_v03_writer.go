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
var _ EventContextWriter = (*EventContextV03)(nil)

// SetSpecVersion implements EventContextWriter.SetSpecVersion
func (ec *EventContextV03) SetSpecVersion(v string) error {
	if v != CloudEventsVersionV03 {
		return fmt.Errorf("invalid version %q, expecting %q", v, CloudEventsVersionV03)
	}
	ec.SpecVersion = CloudEventsVersionV03
	return nil
}

// SetDataContentType implements EventContextWriter.SetDataContentType
func (ec *EventContextV03) SetDataContentType(ct string) error {
	ct = strings.TrimSpace(ct)
	if ct == "" {
		ec.DataContentType = nil
	} else {
		ec.DataContentType = &ct
	}
	return nil
}

// SetType implements EventContextWriter.SetType
func (ec *EventContextV03) SetType(t string) error {
	t = strings.TrimSpace(t)
	ec.Type = t
	return nil
}

// SetSource implements EventContextWriter.SetSource
func (ec *EventContextV03) SetSource(u string) error {
	pu, err := url.Parse(u)
	if err != nil {
		return err
	}
	ec.Source = types.URLRef{URL: *pu}
	return nil
}

// SetSubject implements EventContextWriter.SetSubject
func (ec *EventContextV03) SetSubject(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		ec.Subject = nil
	} else {
		ec.Subject = &s
	}
	return nil
}

// SetID implements EventContextWriter.SetID
func (ec *EventContextV03) SetID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return errors.New("id is required to be a non-empty string")
	}
	ec.ID = id
	return nil
}

// SetTime implements EventContextWriter.SetTime
func (ec *EventContextV03) SetTime(t time.Time) error {
	if t.IsZero() {
		ec.Time = nil
	} else {
		ec.Time = &types.Timestamp{Time: t}
	}
	return nil
}

// SetDataSchema implements EventContextWriter.SetDataSchema
func (ec *EventContextV03) SetDataSchema(u string) error {
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
func (ec *EventContextV03) DeprecatedSetDataContentEncoding(e string) error {
	e = strings.ToLower(strings.TrimSpace(e))
	if e == "" {
		ec.DataContentEncoding = nil
	} else {
		ec.DataContentEncoding = &e
	}
	return nil
}
