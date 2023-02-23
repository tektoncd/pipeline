/*
Copyright 2019 The Tekton Authors

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
package termination

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/logging"
)

func TestParseMessage(t *testing.T) {
	for _, c := range []struct {
		desc, msg string
		want      []v1beta1.PipelineResourceResult
	}{{
		desc: "valid message",
		msg:  `[{"key": "digest","value":"hereisthedigest"},{"key":"foo","value":"bar"}]`,
		want: []v1beta1.PipelineResourceResult{{
			Key:   "digest",
			Value: "hereisthedigest",
		}, {
			Key:   "foo",
			Value: "bar",
		}},
	}, {
		desc: "invalid key in message",
		msg:  `[{"invalid": "digest","value":"hereisthedigest"},{"key":"foo","value":"bar"}]`,
		want: []v1beta1.PipelineResourceResult{{
			Value: "hereisthedigest",
		}, {
			Key:   "foo",
			Value: "bar",
		}},
	}, {
		desc: "empty message",
		msg:  "",
		want: nil,
	}, {
		desc: "duplicate keys",
		msg: `[
		{"key":"foo","value":"first"},
		{"key":"foo","value":"middle"},
		{"key":"foo","value":"last"}]`,
		want: []v1beta1.PipelineResourceResult{{
			Key:   "foo",
			Value: "last",
		}},
	}, {
		desc: "sorted by key",
		msg: `[
		{"key":"zzz","value":"last"},
		{"key":"ddd","value":"middle"},
		{"key":"aaa","value":"first"}]`,
		want: []v1beta1.PipelineResourceResult{{
			Key:   "aaa",
			Value: "first",
		}, {
			Key:   "ddd",
			Value: "middle",
		}, {
			Key:   "zzz",
			Value: "last",
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "status")
			got, err := ParseMessage(logger, c.msg)
			if err != nil {
				t.Fatalf("ParseMessage: %v", err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("ParseMessage %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestParseMessageInvalidMessage(t *testing.T) {
	for _, c := range []struct {
		desc, msg, wantError string
	}{{
		desc:      "invalid JSON",
		msg:       "invalid JSON",
		wantError: "parsing message json",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "status")
			_, err := ParseMessage(logger, c.msg)
			if err == nil {
				t.Errorf("Expected error parsing incorrect termination message, got nil")
			}
			if !strings.HasPrefix(err.Error(), c.wantError) {
				t.Errorf("Expected different error: %s", c.wantError)
			}
		})
	}
}
