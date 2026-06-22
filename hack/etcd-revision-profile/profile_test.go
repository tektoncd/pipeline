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

	got, err := parseEtcdGetJSON([]byte(raw))
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
	_, err := parseEtcdGetJSON(raw)
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
		name string
		ref  objectRef
		want string
	}{{
		name: "tekton CRD (namespaced)",
		ref:  objectRef{Group: "tekton.dev", Resource: "pipelineruns", Namespace: "default", Name: "r1"},
		want: "/registry/tekton.dev/pipelineruns/default/r1",
	}, {
		name: "core pod",
		ref:  objectRef{Group: "", Resource: "pods", Namespace: "default", Name: "r1-build-pod"},
		want: "/registry/pods/default/r1-build-pod",
	}, {
		name: "core event",
		ref:  objectRef{Group: "", Resource: "events", Namespace: "ci", Name: "r1.17abc"},
		want: "/registry/events/ci/r1.17abc",
	}, {
		name: "cluster-scoped (no namespace)",
		ref:  objectRef{Group: "", Resource: "nodes", Namespace: "", Name: "node-1"},
		want: "/registry/nodes/node-1",
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := etcdKeyFor(tc.ref); got != tc.want {
				t.Errorf("etcdKeyFor(%+v) = %q, want %q", tc.ref, got, tc.want)
			}
		})
	}
}

// aggregate groups objects by Kind (sorted), summing the revision count, the
// current value size, and the upper-bound revision-bytes estimate
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
