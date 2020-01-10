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
	"testing"

	"github.com/google/go-cmp/cmp"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func TestParseMessage(t *testing.T) {
	for _, c := range []struct {
		desc, msg string
		want      []v1alpha1.PipelineResourceResult
	}{{
		desc: "valid message",
		msg:  `[{"digest":"foo"},{"key":"foo","value":"bar"}]`,
		want: []v1alpha1.PipelineResourceResult{{
			Digest: "foo",
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
		want: []v1alpha1.PipelineResourceResult{{
			Key:   "foo",
			Value: "last",
		}},
	}, {
		desc: "sorted by key",
		msg: `[
		{"key":"zzz","value":"last"},
		{"key":"ddd","value":"middle"},
		{"key":"aaa","value":"first"}]`,
		want: []v1alpha1.PipelineResourceResult{{
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
			got, err := ParseMessage(c.msg)
			if err != nil {
				t.Fatalf("ParseMessage: %v", err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("ParseMessage(-want,+got): %s", d)
			}
		})
	}
}

func TestParseMessage_Invalid(t *testing.T) {
	if _, err := ParseMessage("INVALID NOT JSON"); err == nil {
		t.Error("Expected error parsing invalid JSON, got nil")
	}
}
