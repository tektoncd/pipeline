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

package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
)

func TestEventFormatIsValid(t *testing.T) {
	for _, tc := range []struct {
		desc   string
		format config.EventFormat
		want   bool
	}{{
		desc:   "Tekton v1",
		format: config.FormatTektonV1,
		want:   true,
	}, {
		desc:   "Invalid",
		format: config.EventFormat("invalid"),
		want:   false,
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.format.IsValid()
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}

func TestEventFormatsString(t *testing.T) {
	for _, tc := range []struct {
		desc    string
		formats config.EventFormats
		want    string
	}{{
		desc:    "Just one",
		formats: config.EventFormats{config.EventFormat("one"): struct{}{}},
		want:    "one",
	}, {
		desc:    "Empty",
		formats: config.EventFormats{},
		want:    "",
	}, {
		desc: "Many",
		formats: config.EventFormats{
			config.EventFormat("a"): struct{}{},
			config.EventFormat("b"): struct{}{},
			config.EventFormat("c"): struct{}{},
		},
		want: "a,b,c",
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.formats.String()
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}

func TestEventFormatsEquals(t *testing.T) {
	for _, tc := range []struct {
		desc  string
		this  config.EventFormats
		other config.EventFormats
		want  bool
	}{{
		desc:  "Both empty",
		this:  config.EventFormats{},
		other: config.EventFormats{},
		want:  true,
	}, {
		desc: "One empty",
		this: config.EventFormats{
			config.EventFormat("one"): struct{}{},
		},
		other: config.EventFormats{},
		want:  false,
	}, {
		desc: "Same size, different",
		this: config.EventFormats{
			config.EventFormat("one"): struct{}{},
		},
		other: config.EventFormats{
			config.EventFormat("two"): struct{}{},
		},
		want: false,
	}, {
		desc: "Different size, first equal",
		this: config.EventFormats{
			config.EventFormat("one"): struct{}{},
		},
		other: config.EventFormats{
			config.EventFormat("one"): struct{}{},
			config.EventFormat("two"): struct{}{},
		},
		want: false,
	}, {
		desc: "Identical, different order",
		this: config.EventFormats{
			config.EventFormat("two"): struct{}{},
			config.EventFormat("one"): struct{}{},
		},
		other: config.EventFormats{
			config.EventFormat("one"): struct{}{},
			config.EventFormat("two"): struct{}{},
		},
		want: true,
	}, {
		desc: "Identical, same order",
		this: config.EventFormats{
			config.EventFormat("one"): struct{}{},
			config.EventFormat("two"): struct{}{},
		},
		other: config.EventFormats{
			config.EventFormat("one"): struct{}{},
			config.EventFormat("two"): struct{}{},
		},
		want: true,
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			got := tc.this.Equals(tc.other)
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}

func TestParseEventFormats(t *testing.T) {
	for _, tc := range []struct {
		desc      string
		formats   string
		want      config.EventFormats
		wantError bool
	}{{
		desc:      "Empty string",
		formats:   "",
		want:      config.EventFormats{},
		wantError: true,
	}, {
		desc:      "Invalid value",
		formats:   "foobar",
		want:      config.EventFormats{},
		wantError: true,
	}, {
		desc:      "One valid, one invalid",
		formats:   "tektonv1,foobar",
		want:      config.EventFormats{},
		wantError: true,
	}, {
		desc:      "One invalid, one valid",
		formats:   "foobar,tektonv1",
		want:      config.EventFormats{},
		wantError: true,
	}, {
		desc:      "One valid",
		formats:   "tektonv1",
		want:      config.EventFormats{config.FormatTektonV1: struct{}{}},
		wantError: false,
	}, {
		desc:      "Two valid, duplicate",
		formats:   "tektonv1,tektonv1",
		want:      config.EventFormats{},
		wantError: true,
	}} {
		t.Run(tc.desc, func(t *testing.T) {
			got, err := config.ParseEventFormats(tc.formats)
			if d := cmp.Diff(tc.wantError, err != nil); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}

func TestNewEventsFromConfigMap(t *testing.T) {
	for _, tc := range []struct {
		description    string
		expectedConfig *config.Events
		expectedError  bool
		fileName       string
	}{{
		description: "get events config name",
		expectedConfig: &config.Events{
			Formats: config.EventFormats{
				config.FormatTektonV1: struct{}{},
			},
			Sink: "http://events.sink",
		},
		fileName: config.GetEventsConfigName(),
	}, {
		description: "test defaults",
		expectedConfig: &config.Events{
			Formats: config.DefaultFormats,
			Sink:    config.DefaultSink,
		},
		fileName: "config-events-empty",
	}, {
		description:   "empty values in formats",
		expectedError: true,
		fileName:      "config-events-empty-values",
	}, {
		description:   "invalid in config map",
		expectedError: true,
		fileName:      "config-events-error",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			cm := test.ConfigMapFromTestFile(t, tc.fileName)
			events, err := config.NewEventsFromConfigMap(cm)
			if d := cmp.Diff(tc.expectedError, err != nil); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
			if d := cmp.Diff(tc.expectedConfig, events); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}

func TestEventsConfigEquals(t *testing.T) {
	for _, tc := range []struct {
		name     string
		left     *config.Events
		right    *config.Events
		expected bool
	}{{
		name:     "left and right nil",
		left:     nil,
		right:    nil,
		expected: true,
	}, {
		name:     "left nil",
		left:     nil,
		right:    &config.Events{},
		expected: false,
	}, {
		name:     "right nil",
		left:     &config.Events{},
		right:    nil,
		expected: false,
	}, {
		name:     "left and right default",
		left:     &config.Events{},
		right:    &config.Events{},
		expected: true,
	}, {
		name: "different format",
		left: &config.Events{
			Formats: config.EventFormats{
				config.EventFormat("foo"): struct{}{},
			},
			Sink: "http://event.sink",
		},
		right: &config.Events{
			Formats: config.EventFormats{
				config.EventFormat("bar"): struct{}{},
			},
			Sink: "http://event.sink",
		},
		expected: false,
	}, {
		name: "different sink",
		left: &config.Events{
			Formats: config.EventFormats{
				config.EventFormat("foo"): struct{}{},
			},
			Sink: "http://event.sink/1",
		},
		right: &config.Events{
			Formats: config.EventFormats{
				config.EventFormat("foo"): struct{}{},
			},
			Sink: "http://event.sink/2",
		},
		expected: false,
	}, {
		name: "identical",
		left: &config.Events{
			Formats: config.EventFormats{
				config.EventFormat("foo"): struct{}{},
			},
			Sink: "http://event.sink",
		},
		right: &config.Events{
			Formats: config.EventFormats{
				config.EventFormat("foo"): struct{}{},
			},
			Sink: "http://event.sink",
		},
		expected: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			actual := tc.left.Equals(tc.right)
			if d := cmp.Diff(tc.expected, actual); d != "" {
				t.Errorf("Diff(-want,+got):\n%s", d)
			}
		})
	}
}
