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
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// etcdObject is the etcd MVCC footprint of one stored Kubernetes object.
//
// Version is etcd's per-key write counter: it starts at 1 when the key is
// created and increments on every write, so it equals the number of revisions
// the object has accumulated since creation. ValueBytes is the size of the
// object's current serialized value (the live snapshot), not the sum across
// revisions.
type etcdObject struct {
	Key        string
	Kind       string
	Namespace  string
	Name       string
	Version    int64
	ValueBytes int
}

// objectRef identifies a Kubernetes object well enough to construct its
// apiserver storage key. Group is "" for the core API group.
type objectRef struct {
	Group     string
	Resource  string
	Namespace string
	Name      string
}

// Object coordinates shared by discovery and key construction.
const (
	kindPipelineRun = "PipelineRun"
	kindTaskRun     = "TaskRun"
	kindPod         = "Pod"
	kindEvent       = "Event"

	groupTektonDev = "tekton.dev"

	resPipelineRuns = "pipelineruns"
	resTaskRuns     = "taskruns"
	resPods         = "pods"
	resEvents       = "events"

	totalRow = "TOTAL"
)

// etcdKeyFor reproduces the apiserver's etcd storage key layout:
//
//	core group:  /registry/<resource>/<namespace>/<name>
//	other group: /registry/<group>/<resource>/<namespace>/<name>
//
// Cluster-scoped objects (empty Namespace) drop the namespace segment.
//
// Note: a handful of built-in core resources use legacy prefixes that do not
// match their API name (most notably Node, stored under /registry/minions/).
// Callers profiling those must pass the storage name in Resource.
func etcdKeyFor(ref objectRef) string {
	parts := []string{"registry"}
	if ref.Group != "" {
		parts = append(parts, ref.Group)
	}
	parts = append(parts, ref.Resource)
	if ref.Namespace != "" {
		parts = append(parts, ref.Namespace)
	}
	parts = append(parts, ref.Name)
	return "/" + strings.Join(parts, "/")
}

// etcdGetResponse is the subset of `etcdctl get <key> -w json` we consume.
type etcdGetResponse struct {
	Count int `json:"count"`
	Kvs   []struct {
		Key     string `json:"key"`
		Version int64  `json:"version"`
		Value   string `json:"value"`
	} `json:"kvs"`
}

// parseEtcdGetJSON parses the JSON emitted by `etcdctl get <key> -w json` for a
// single key and returns its decoded key, version, and current value size.
// A response with no kvs (count 0) is reported as an error rather than a
// zero-valued success, since that means the key does not exist.
func parseEtcdGetJSON(raw []byte) (etcdObject, error) {
	var resp etcdGetResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return etcdObject{}, fmt.Errorf("decoding etcdctl json: %w", err)
	}
	if len(resp.Kvs) == 0 {
		return etcdObject{}, fmt.Errorf("%w (count=%d)", errNotFound, resp.Count)
	}
	kv := resp.Kvs[0]
	key, err := base64.StdEncoding.DecodeString(kv.Key)
	if err != nil {
		return etcdObject{}, fmt.Errorf("decoding key: %w", err)
	}
	value, err := base64.StdEncoding.DecodeString(kv.Value)
	if err != nil {
		return etcdObject{}, fmt.Errorf("decoding value: %w", err)
	}
	return etcdObject{
		Key:        string(key),
		Version:    kv.Version,
		ValueBytes: len(value),
	}, nil
}

// kindProfile aggregates the etcd footprint of every object of one Kind.
type kindProfile struct {
	Kind             string
	Count            int
	TotalRevisions   int64 // sum of per-object Version
	CurrentBytes     int64 // sum of current value sizes (live snapshot)
	EstRevisionBytes int64 // sum of Version*ValueBytes, a rough estimate of
	// pre-compaction storage assuming each revision is about the current size
}

// profile is the full per-Kind breakdown plus a grand total.
type profile struct {
	Kinds []kindProfile
	Total kindProfile
}

// aggregate buckets objects by Kind (rows sorted by Kind name) and computes the
// per-Kind and overall revision/size totals.
func aggregate(objs []etcdObject) profile {
	byKind := map[string]*kindProfile{}
	for _, o := range objs {
		kp := byKind[o.Kind]
		if kp == nil {
			kp = &kindProfile{Kind: o.Kind}
			byKind[o.Kind] = kp
		}
		kp.Count++
		kp.TotalRevisions += o.Version
		kp.CurrentBytes += int64(o.ValueBytes)
		kp.EstRevisionBytes += o.Version * int64(o.ValueBytes)
	}

	kinds := make([]string, 0, len(byKind))
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)

	p := profile{Total: kindProfile{Kind: totalRow}}
	for _, k := range kinds {
		kp := byKind[k]
		p.Kinds = append(p.Kinds, *kp)
		p.Total.Count += kp.Count
		p.Total.TotalRevisions += kp.TotalRevisions
		p.Total.CurrentBytes += kp.CurrentBytes
		p.Total.EstRevisionBytes += kp.EstRevisionBytes
	}
	return p
}

// renderTable formats a profile as a fixed-width text table.
func renderTable(p profile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-14s %6s %10s %14s %18s\n", "KIND", "COUNT", "REVISIONS", "CURRENT(B)", "EST-REV-BYTES(B)")
	row := func(kp kindProfile) {
		fmt.Fprintf(&b, "%-14s %6d %10d %14d %18d\n",
			kp.Kind, kp.Count, kp.TotalRevisions, kp.CurrentBytes, kp.EstRevisionBytes)
	}
	for _, kp := range p.Kinds {
		row(kp)
	}
	fmt.Fprintf(&b, "%s\n", strings.Repeat("-", 66))
	row(p.Total)
	return b.String()
}
