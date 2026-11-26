/*
Copyright 2026 The Tekton Authors

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

package main

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

// parseEtcdGetJSON must understand the real shape emitted by
// `etcdctl get <key> -w json`: a header plus a kvs array whose key/value are
// base64-encoded and whose `version` is etcd's per-key write count.
func TestParseEtcdGetJSON(t *testing.T) {
	key := "/registry/tekton.dev/taskruns/default/demo-run-build"
	value := []byte("0123456789") // 10 bytes once base64-decoded

	raw := fmt.Sprintf(
		`{"header":{"revision":42},"kvs":[{"key":%q,"create_revision":6,"mod_revision":40,"version":17,"value":%q}],"count":1}`,
		base64.StdEncoding.EncodeToString([]byte(key)),
		base64.StdEncoding.EncodeToString(value),
	)

	got, err := parseEtcdGetJSON([]byte(raw), key)
	if err != nil {
		t.Fatalf("parseEtcdGetJSON() unexpected error: %v", err)
	}
	if got.Key != key {
		t.Errorf("Key = %q, want %q", got.Key, key)
	}
	if got.Version != 17 {
		t.Errorf("Version = %d, want 17", got.Version)
	}
	if got.ValueBytes != len(value) {
		t.Errorf("ValueBytes = %d, want %d", got.ValueBytes, len(value))
	}
}

// A get against a non-existent key returns count:0 and an empty kvs array;
// that must be surfaced as an error, not a zero-valued success.
func TestParseEtcdGetJSON_NotFound(t *testing.T) {
	raw := []byte(`{"header":{"revision":42},"count":0}`)
	_, err := parseEtcdGetJSON(raw, "/registry/pods/ci/gone")
	if err == nil {
		t.Fatal("parseEtcdGetJSON() on count:0 = nil error, want an error")
	}
	if !errors.Is(err, errNotFound) {
		t.Errorf("error %v cannot be detected as not-found via errors.Is(errNotFound)", err)
	}
}

// etcdKeyFor must reproduce the apiserver storage key layout: core resources
// have no group segment, CRDs and other API groups do.
func TestEtcdKeyFor(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		ref    objectRef
		want   string
	}{{
		name:   "tekton CRD (namespaced)",
		prefix: defaultEtcdPrefix,
		ref:    objectRef{Group: "tekton.dev", Resource: "pipelineruns", Namespace: "default", Name: "r1"},
		want:   "/registry/tekton.dev/pipelineruns/default/r1",
	}, {
		name:   "core pod",
		prefix: defaultEtcdPrefix,
		ref:    objectRef{Group: "", Resource: "pods", Namespace: "default", Name: "r1-build-pod"},
		want:   "/registry/pods/default/r1-build-pod",
	}, {
		name:   "core event",
		prefix: defaultEtcdPrefix,
		ref:    objectRef{Group: "", Resource: "events", Namespace: "ci", Name: "r1.17abc"},
		want:   "/registry/events/ci/r1.17abc",
	}, {
		name:   "cluster-scoped (no namespace)",
		prefix: defaultEtcdPrefix,
		ref:    objectRef{Group: "", Resource: "nodes", Namespace: "", Name: "node-1"},
		want:   "/registry/nodes/node-1",
	}, {
		// --etcd-prefix is configurable; a cluster that changed it stores every
		// object somewhere other than /registry.
		name:   "custom apiserver prefix",
		prefix: "/tekton-registry",
		ref:    objectRef{Group: "tekton.dev", Resource: "taskruns", Namespace: "ci", Name: "t1"},
		want:   "/tekton-registry/tekton.dev/taskruns/ci/t1",
	}, {
		name:   "prefix with surrounding slashes is normalized",
		prefix: "/nested/registry/",
		ref:    objectRef{Group: "", Resource: "pods", Namespace: "ci", Name: "p1"},
		want:   "/nested/registry/pods/ci/p1",
	}, {
		// The flag supplies the /registry default, so an explicit root or
		// empty prefix means the store root, as the apiserver resolves it.
		name:   "explicit root prefix is honoured",
		prefix: "/",
		ref:    objectRef{Group: "", Resource: "pods", Namespace: "ci", Name: "p1"},
		want:   "/pods/ci/p1",
	}, {
		name:   "empty prefix is the store root",
		prefix: "",
		ref:    objectRef{Group: "tekton.dev", Resource: "taskruns", Namespace: "ci", Name: "t1"},
		want:   "/tekton.dev/taskruns/ci/t1",
	}, {
		name:   "prefix without a leading slash still resolves",
		prefix: "registry",
		ref:    objectRef{Group: "", Resource: "pods", Namespace: "ci", Name: "p1"},
		want:   "/registry/pods/ci/p1",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := etcdKeyFor(tc.prefix, tc.ref); got != tc.want {
				t.Errorf("etcdKeyFor(%q, %+v) = %q, want %q", tc.prefix, tc.ref, got, tc.want)
			}
		})
	}
}

// aggregate groups objects by Kind (sorted), summing the revision count, the
// current value size, and the lifetime revision-bytes estimate
// (sum of version*size), and produces a grand total.
func TestAggregate(t *testing.T) {
	objs := []etcdObject{
		{Kind: "PipelineRun", Version: 9, ValueBytes: 4000},
		{Kind: "TaskRun", Version: 14, ValueBytes: 68000},
		{Kind: "TaskRun", Version: 11, ValueBytes: 64000},
		{Kind: "Pod", Version: 8, ValueBytes: 12000},
		{Kind: "Pod", Version: 6, ValueBytes: 11000},
		{Kind: "Event", Version: 1, ValueBytes: 600},
		{Kind: "Event", Version: 1, ValueBytes: 600},
		{Kind: "Event", Version: 1, ValueBytes: 600},
	}

	got := aggregate(objs)

	// Rows are sorted by Kind name: "PipelineRun" sorts before "Pod"
	// (they differ first at the 2nd character, 'i' < 'o').
	wantKinds := []kindProfile{
		{Kind: "Event", Count: 3, TotalRevisions: 3, CurrentBytes: 1800, EstRevisionBytes: 1800},
		{Kind: "PipelineRun", Count: 1, TotalRevisions: 9, CurrentBytes: 4000, EstRevisionBytes: 36000},
		{Kind: "Pod", Count: 2, TotalRevisions: 14, CurrentBytes: 23000, EstRevisionBytes: 162000},
		{Kind: "TaskRun", Count: 2, TotalRevisions: 25, CurrentBytes: 132000, EstRevisionBytes: 1656000},
	}
	if len(got.Kinds) != len(wantKinds) {
		t.Fatalf("got %d kind rows, want %d (%+v)", len(got.Kinds), len(wantKinds), got.Kinds)
	}
	for i, w := range wantKinds {
		if got.Kinds[i] != w {
			t.Errorf("Kinds[%d] = %+v, want %+v", i, got.Kinds[i], w)
		}
	}

	wantTotal := kindProfile{Kind: "TOTAL", Count: 8, TotalRevisions: 51, CurrentBytes: 160800, EstRevisionBytes: 1855800}
	if got.Total != wantTotal {
		t.Errorf("Total = %+v, want %+v", got.Total, wantTotal)
	}
}

func TestRenderTableContainsTotals(t *testing.T) {
	p := aggregate([]etcdObject{
		{Kind: "PipelineRun", Version: 9, ValueBytes: 4000},
		{Kind: "TaskRun", Version: 14, ValueBytes: 68000},
	})
	out := renderTable(p)
	for _, want := range []string{"PipelineRun", "TaskRun", "TOTAL", "REVISIONS"} {
		if !strings.Contains(out, want) {
			t.Errorf("renderTable() output missing %q\n%s", want, out)
		}
	}
}

// The name column has to size itself: real Pod and Event names carry the
// PipelineRun's name plus a suffix and overflow any fixed width, which bends
// every column to their right.
func TestRenderObjectsColumnsLineUp(t *testing.T) {
	out := renderObjects(aggregate([]etcdObject{
		{Kind: "PipelineRun", Name: "etcd-demo-fm98n", Version: 6, ValueBytes: 3887},
		{Kind: "Pod", Name: "etcd-demo-fm98n-package-pod", Version: 14, ValueBytes: 10798},
		{Kind: "Event", Name: "etcd-demo-fm98n-build.1870f3a4c1b2d3e4", Version: 1, ValueBytes: 600},
	}))

	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("got %d lines, want a header and 3 rows:\n%s", len(lines), out)
	}
	// Every column is fixed width and the numbers are right aligned, so an
	// aligned table has rows exactly as wide as its header.
	for i, l := range lines[1:] {
		if len(l) != len(lines[0]) {
			t.Errorf("row %d is %d columns wide, header is %d\n%s", i, len(l), len(lines[0]), out)
		}
	}
}

// The header revision is what every child read is pinned to, so losing it
// would silently turn the profile back into a rolling read.
func TestParseEtcdGetJSONKeepsHeaderRevision(t *testing.T) {
	raw := fmt.Sprintf(`{"header":{"revision":4242},"kvs":[{"key":%q,"version":3,"value":%q}],"count":1}`,
		base64.StdEncoding.EncodeToString([]byte("/registry/pods/ci/p")),
		base64.StdEncoding.EncodeToString([]byte("v")))
	got, err := parseEtcdGetJSON([]byte(raw), "/registry/pods/ci/p")
	if err != nil {
		t.Fatalf("parseEtcdGetJSON() error: %v", err)
	}
	if got.headerRevision != 4242 {
		t.Errorf("headerRevision = %d, want 4242", got.headerRevision)
	}
}

// Every later read is pinned to the revision this response reports, and etcd
// reads a revision of zero as "latest". A response that cannot supply one, or
// that answers about some other key, has to be an error rather than something
// the caller silently builds a rolling profile from.
func TestParseEtcdGetJSONRejectsUnusableResponses(t *testing.T) {
	const key = "/registry/pods/ci/p"
	enc := base64.StdEncoding.EncodeToString
	kv := func(k string, version int) string {
		return fmt.Sprintf(`{"key":%q,"version":%d,"value":%q}`, enc([]byte(k)), version, enc([]byte("v")))
	}

	tests := []struct {
		name string
		raw  string
	}{
		{"no header revision", `{"kvs":[` + kv(key, 3) + `],"count":1}`},
		{"zero header revision", `{"header":{"revision":0},"kvs":[` + kv(key, 3) + `],"count":1}`},
		{"count disagrees with kvs", `{"header":{"revision":9},"kvs":[` + kv(key, 3) + `],"count":2}`},
		{"more than one kv", `{"header":{"revision":9},"kvs":[` + kv(key, 3) + `,` + kv("/registry/pods/ci/q", 3) + `],"count":2}`},
		{"answers about another key", `{"header":{"revision":9},"kvs":[` + kv("/registry/pods/ci/other", 3) + `],"count":1}`},
		{"live key reporting version zero", `{"header":{"revision":9},"kvs":[` + kv(key, 0) + `],"count":1}`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseEtcdGetJSON([]byte(tc.raw), key); err == nil {
				t.Errorf("parseEtcdGetJSON() = nil error for %s", tc.name)
			}
		})
	}
}

// An empty kvs array is a normal not-found answer only when count is zero. A
// response that claims a key and returns none is inconsistent, and treating it
// as not-found would quietly demote it to a skipped object.
func TestParseEtcdGetJSONCountWithoutKvsIsNotNotFound(t *testing.T) {
	_, err := parseEtcdGetJSON([]byte(`{"header":{"revision":42},"count":1,"kvs":[]}`), "/registry/pods/ci/p")
	if err == nil {
		t.Fatal("parseEtcdGetJSON() = nil error for count=1 with no kvs")
	}
	if errors.Is(err, errNotFound) {
		t.Errorf("error %v reports not-found; an inconsistent response must abort instead", err)
	}
}

// etcd answers a missing key with a valid header and no kvs, so a response
// carrying neither is not a missing key: it is an answer that cannot be
// trusted, and must abort rather than be skipped as merely absent.
func TestParseEtcdGetJSONEmptyResponseIsNotNotFound(t *testing.T) {
	for _, raw := range []string{`{}`, `{"count":0}`} {
		_, err := parseEtcdGetJSON([]byte(raw), "/registry/pods/ci/p")
		if err == nil {
			t.Errorf("parseEtcdGetJSON(%s) = nil error", raw)
			continue
		}
		if errors.Is(err, errNotFound) {
			t.Errorf("parseEtcdGetJSON(%s) reports not-found; a response with no header cannot establish that", raw)
		}
	}

	// A real missing key does carry a header, and must still be not-found.
	_, err := parseEtcdGetJSON([]byte(`{"header":{"revision":42},"count":0}`), "/registry/pods/ci/p")
	if !errors.Is(err, errNotFound) {
		t.Errorf("a genuine missing key reported %v, want not-found", err)
	}
}
