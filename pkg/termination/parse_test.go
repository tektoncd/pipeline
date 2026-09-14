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
package termination_test

import (
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/result"
	termination "github.com/tektoncd/pipeline/pkg/termination"
	"github.com/tektoncd/pipeline/test/diff"
	"knative.dev/pkg/logging"
)

func TestParseMessage(t *testing.T) {
	for _, c := range []struct {
		desc, msg string
		want      []result.RunResult
	}{{
		desc: "valid message",
		msg:  `[{"key": "digest","value":"hereisthedigest"},{"key":"foo","value":"bar"}]`,
		want: []result.RunResult{{
			Key:   "digest",
			Value: "hereisthedigest",
		}, {
			Key:   "foo",
			Value: "bar",
		}},
	}, {
		desc: "invalid key in message",
		msg:  `[{"invalid": "digest","value":"hereisthedigest"},{"key":"foo","value":"bar"}]`,
		want: []result.RunResult{{
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
		want: []result.RunResult{{
			Key:   "foo",
			Value: "last",
		}},
	}, {
		desc: "duplicate keys with incorrect result",
		msg: `[
		{"key":"foo","value":"first"},
		{},
		{"key":"foo","value":"middle"},
		{"key":"foo","value":"last"}]`,
		want: []result.RunResult{{
			Key:   "foo",
			Value: "last",
		}},
	}, {
		desc: "sorted by key",
		msg: `[
		{"key":"zzz","value":"last"},
		{"key":"ddd","value":"middle"},
		{"key":"aaa","value":"first"}]`,
		want: []result.RunResult{{
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
			got, err := termination.ParseMessage(logger, c.msg)
			if err != nil {
				t.Fatalf("ParseMessage: %v", err)
			}
			if d := cmp.Diff(c.want, got); d != "" {
				t.Fatalf("ParseMessage %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestParseMessage_CompressedAutoDetect(t *testing.T) {
	logger, _ := logging.NewLogger("", "status")

	// Write compressed results to a temp file, then read back and parse.
	tmpFile, err := os.CreateTemp(t.TempDir(), "compressedMsg")
	if err != nil {
		t.Fatalf("Cannot create temporary file: %v", err)
	}

	// Use enough results so compression actually saves space and produces tknz: prefix
	var want []result.RunResult
	for i := range 20 {
		want = append(want, result.RunResult{
			Key:        "image-digest-" + string(rune('a'+i)),
			Value:      "gcr.io/project/image@sha256:abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
			ResultType: result.TaskRunResultType,
		})
	}

	if err := termination.WriteCompressedMessage(tmpFile.Name(), want); err != nil {
		t.Fatalf("WriteCompressedMessage: %v", err)
	}

	fileContents, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}

	// Verify the raw content starts with the compressed prefix.
	if !strings.HasPrefix(string(fileContents), "tknz:") {
		t.Fatalf("Expected compressed prefix 'tknz:', got: %.40s...", string(fileContents))
	}

	got, err := termination.ParseMessage(logger, string(fileContents))
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	// Sort want to match ParseMessage's sorting
	sort.Slice(want, func(i, j int) bool { return want[i].Key < want[j].Key })
	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("ParseMessage compressed auto-detect %s", diff.PrintWantGot(d))
	}
}

func TestParseMessage_PlainJSON_StillWorks(t *testing.T) {
	logger, _ := logging.NewLogger("", "status")

	msg := `[{"key":"digest","value":"sha256:abc123","type":1},{"key":"url","value":"https://example.com","type":1}]`

	want := []result.RunResult{{
		Key:        "digest",
		Value:      "sha256:abc123",
		ResultType: result.TaskRunResultType,
	}, {
		Key:        "url",
		Value:      "https://example.com",
		ResultType: result.TaskRunResultType,
	}}

	got, err := termination.ParseMessage(logger, msg)
	if err != nil {
		t.Fatalf("ParseMessage: %v", err)
	}

	if d := cmp.Diff(want, got); d != "" {
		t.Fatalf("ParseMessage plain JSON backward compat %s", diff.PrintWantGot(d))
	}
}

func TestParseMessageInvalidMessage(t *testing.T) {
	for _, c := range []struct {
		desc, msg, wantError string
	}{{
		desc:      "invalid JSON",
		msg:       "invalid JSON",
		wantError: "parsing message json",
	}, {
		desc:      "compressed prefix with invalid base64",
		msg:       "tknz:not-valid-base64!!!",
		wantError: "decompressing termination message",
	}, {
		desc:      "compressed prefix with valid base64 but invalid flate",
		msg:       "tknz:aGVsbG8gd29ybGQ",
		wantError: "decompressing termination message",
	}, {
		desc:      "compressed prefix with empty payload",
		msg:       "tknz:",
		wantError: "decompressing termination message",
	}} {
		t.Run(c.desc, func(t *testing.T) {
			logger, _ := logging.NewLogger("", "status")
			_, err := termination.ParseMessage(logger, c.msg)
			if err == nil {
				t.Errorf("Expected error parsing incorrect termination message, got nil")
			}
			if !strings.HasPrefix(err.Error(), c.wantError) {
				t.Errorf("Expected different error: %s", c.wantError)
			}
		})
	}
}
