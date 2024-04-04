/*
Copyright 2023 The Tekton Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"errors"
	"os"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

const (
	// FormatTektonV1 represents the "v1" events in Tekton custom format
	FormatTektonV1 EventFormat = "tektonv1"

	// DefaultSink is the default value for "sink"
	DefaultSink = ""

	formatsKey = "formats"
	sinkKey    = "sink"
)

var (
	// TODO(afrittoli): only one valid format for now, more to come
	// See TEP-0137 https://github.com/tektoncd/community/pull/1028
	validFormats = EventFormats{FormatTektonV1: struct{}{}}

	// DefaultFormat is the default value for "formats"
	DefaultFormats = EventFormats{FormatTektonV1: struct{}{}}

	// DefaultConfig holds all the default configurations for the config.
	DefaultEvents, _ = NewEventsFromMap(map[string]string{})
)

// Events holds the events configurations
// +k8s:deepcopy-gen=true
type Events struct {
	Sink    string
	Formats EventFormats
}

// EventFormat is a single event format
type EventFormat string

// EventFormats is a set of event formats
type EventFormats map[EventFormat]struct{}

// String is a string representation of an EventFormat
func (ef EventFormat) String() string {
	return string(ef)
}

// IsValid returns true is the EventFormat one of the valid ones
func (ef EventFormat) IsValid() bool {
	_, ok := validFormats[ef]
	return ok
}

// String is a string representation of an EventFormats
func (efs EventFormats) String() string {
	// Make an array of map keys
	keys := make([]string, len(efs))

	i := 0
	for k := range efs {
		keys[i] = k.String()
		i++
	}
	// Sorting helps with testing
	sort.Strings(keys)

	// Build a comma separated list
	return strings.Join(keys, ",")
}

// Equals defines identity between EventFormats
func (efs EventFormats) Equals(other EventFormats) bool {
	if len(efs) != len(other) {
		return false
	}
	for key := range efs {
		if _, ok := other[key]; !ok {
			return false
		}
	}
	return true
}

// ParseEventFormats converts a comma separated list into a EventFormats set
func ParseEventFormats(formats string) (EventFormats, error) {
	// An empty string is not a valid configuration
	if formats == "" {
		return EventFormats{}, errors.New("formats cannot be empty")
	}
	stringFormats := strings.Split(formats, ",")
	var eventFormats EventFormats = make(map[EventFormat]struct{}, len(stringFormats))
	for _, format := range stringFormats {
		if !EventFormat(format).IsValid() {
			return EventFormats{}, errors.New("invalid format: " + format)
		}
		// If already in the map (duplicate), fail
		if _, ok := eventFormats[EventFormat(format)]; ok {
			return EventFormats{}, errors.New("duplicate format: " + format)
		}
		eventFormats[EventFormat(format)] = struct{}{}
	}
	return eventFormats, nil
}

// GetEventsConfigName returns the name of the configmap containing all
// feature flags.
func GetEventsConfigName() string {
	if e := os.Getenv("CONFIG_EVENTS_NAME"); e != "" {
		return e
	}
	return "config-events"
}

// NewEventsFromMap returns a Config given a map corresponding to a ConfigMap
func NewEventsFromMap(cfgMap map[string]string) (*Events, error) {
	// for any string field with no extra validation
	setField := func(key string, defaultValue string, field *string) {
		if cfg, ok := cfgMap[key]; ok {
			*field = cfg
		} else {
			*field = defaultValue
		}
	}

	events := Events{}
	err := setFormats(cfgMap, DefaultFormats, &events.Formats)
	if err != nil {
		return nil, err
	}
	setField(sinkKey, DefaultSink, &events.Sink)
	return &events, nil
}

func setFormats(cfgMap map[string]string, defaultValue EventFormats, field *EventFormats) error {
	value := defaultValue
	if cfg, ok := cfgMap[formatsKey]; ok {
		v, err := ParseEventFormats(cfg)
		if err != nil {
			return err
		}
		value = v
	}
	*field = value
	return nil
}

// NewEventsFromConfigMap returns a Config for the given configmap
func NewEventsFromConfigMap(config *corev1.ConfigMap) (*Events, error) {
	return NewEventsFromMap(config.Data)
}

// Equals returns true if two Configs are identical
func (cfg *Events) Equals(other *Events) bool {
	if cfg == nil && other == nil {
		return true
	}

	if cfg == nil || other == nil {
		return false
	}

	return other.Sink == cfg.Sink &&
		other.Formats.Equals(cfg.Formats)
}
