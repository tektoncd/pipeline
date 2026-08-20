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
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

// etcdObject is one stored Kubernetes object's write count and current size.
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
	UID        string
	Version    int64
	ValueBytes int

	// value is the raw stored bytes, kept only long enough to confirm the key
	// still holds the object that discovery found. etcd keys carry no UID, so
	// this is the only way to notice a delete and recreate under the same name.
	value []byte
	// headerRevision is the store-wide revision the read was served at.
	headerRevision int64
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
	kindCustomRun   = "CustomRun"

	groupTektonDev = "tekton.dev"

	resPipelineRuns = "pipelineruns"
	resTaskRuns     = "taskruns"
	resPods         = "pods"
	resEvents       = "events"
	resCustomRuns   = "customruns"

	totalRow = "TOTAL"
)

// defaultEtcdPrefix is the kube-apiserver default for --etcd-prefix.
const defaultEtcdPrefix = "/registry"

// etcdKeyFor builds an object's storage key:
//
//	<prefix>/<group>/<resource>/<namespace>/<name>, group omitted when empty
//
// prefix is the apiserver's --etcd-prefix. The flag supplies its /registry
// default, so an explicitly empty or root prefix is honoured as the store root
// rather than quietly turned back into /registry, which is how the apiserver
// itself resolves it. Cluster-scoped objects (empty Namespace) drop the
// namespace segment.
//
// Group is not simply the API group. Resources served by CRDs, Tekton's
// included, are stored under their group, while most compiled-in groups
// register a bare per-resource prefix, so Deployments live at
// /registry/deployments rather than under /registry/apps. The exceptions go
// both ways: apiextensions.k8s.io and apiregistration.k8s.io keep their group,
// Node is stored at /registry/minions, and Services split across
// /registry/services/specs and /registry/services/endpoints. For anything
// beyond Tekton's own resources, Pods and Events, confirm the layout with
// `etcdctl get <prefix> --prefix --keys-only` rather than deriving it.
func etcdKeyFor(prefix string, ref objectRef) string {
	parts := []string{"/", prefix}
	if ref.Group != "" {
		parts = append(parts, ref.Group)
	}
	parts = append(parts, ref.Resource)
	if ref.Namespace != "" {
		parts = append(parts, ref.Namespace)
	}
	parts = append(parts, ref.Name)
	return path.Join(parts...)
}

// etcdGetResponse is the subset of `etcdctl get <key> -w json` we consume.
type etcdGetResponse struct {
	Header struct {
		Revision int64 `json:"revision"`
	} `json:"header"`
	Count int `json:"count"`
	Kvs   []struct {
		Key     string `json:"key"`
		Version int64  `json:"version"`
		Value   string `json:"value"`
	} `json:"kvs"`
}

// parseEtcdGetJSON parses the JSON emitted by `etcdctl get <key> -w json` for
// the single key that was asked for, returning its decoded key, version, and
// current value size. A response with no kvs (count 0) is reported as
// errNotFound rather than a zero-valued success, since that means the key does
// not exist.
//
// The other invariants are checked because the caller pins every later read to
// the revision this response reports. A response without one would leave that
// zero, which etcd reads as "latest", quietly turning the pinned reads back
// into a rolling one.
func parseEtcdGetJSON(raw []byte, wantKey string) (etcdObject, error) {
	var resp etcdGetResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return etcdObject{}, fmt.Errorf("decoding etcdctl json: %w", err)
	}
	// The header comes first: etcd answers a missing key with a valid header and
	// no kvs, so a response without one is not a missing key but an answer that
	// cannot be trusted, and must not be skippable as merely absent.
	if resp.Header.Revision <= 0 {
		return etcdObject{}, fmt.Errorf("etcdctl response for %s carries no header revision", wantKey)
	}
	if resp.Count == 0 && len(resp.Kvs) == 0 {
		return etcdObject{}, fmt.Errorf("%w", errNotFound)
	}
	if len(resp.Kvs) != 1 || resp.Count != 1 {
		return etcdObject{}, fmt.Errorf("etcdctl returned %d key(s) (count=%d) for the single key %s", len(resp.Kvs), resp.Count, wantKey)
	}
	kv := resp.Kvs[0]
	if kv.Version < 1 {
		return etcdObject{}, fmt.Errorf("etcdctl reports version %d for the live key %s", kv.Version, wantKey)
	}
	key, err := base64.StdEncoding.DecodeString(kv.Key)
	if err != nil {
		return etcdObject{}, fmt.Errorf("decoding key: %w", err)
	}
	if string(key) != wantKey {
		return etcdObject{}, fmt.Errorf("etcdctl answered for %s, not the requested %s", key, wantKey)
	}
	value, err := base64.StdEncoding.DecodeString(kv.Value)
	if err != nil {
		return etcdObject{}, fmt.Errorf("decoding value: %w", err)
	}
	return etcdObject{
		Key:            string(key),
		Version:        kv.Version,
		ValueBytes:     len(value),
		value:          value,
		headerRevision: resp.Header.Revision,
	}, nil
}

// holdsUID reports whether the stored value still belongs to the object with
// this UID. Every Kubernetes object serializes metadata.uid as a plain string,
// in both the JSON used for CRDs and the protobuf used for core resources, so
// a substring check works without decoding either encoding. It cannot see
// through encryption at rest, where no plaintext UID is present.
func (o etcdObject) holdsUID(uid string) bool {
	return bytes.Contains(o.value, []byte(uid))
}

// kindProfile aggregates the etcd footprint of every object of one Kind.
type kindProfile struct {
	Kind             string
	Count            int
	TotalRevisions   int64 // sum of per-object Version
	CurrentBytes     int64 // sum of current value sizes (live snapshot)
	EstRevisionBytes int64 // sum of Version*ValueBytes: the serialized volume
	// written over each key's life, assuming every revision was about the size
	// of the current one. It over-counts objects that grew and under-counts
	// ones that shrank, and compaction reclaims history without resetting
	// Version, so it is not the currently retained etcd footprint.
}

// profile is every profiled object, the per-Kind breakdown, and a grand total.
// The per-object rows are what answers "which TaskRun is driving the writes",
// so they are kept rather than only their sums.
type profile struct {
	// Revision is the store-wide revision every key was read at. It cannot be
	// taken from a row: only the PipelineRun is read at latest, and a read
	// pinned with --rev still reports the current store revision in its header.
	Revision int64
	Objects  []etcdObject
	Kinds    []kindProfile
	Total    kindProfile
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

	rows := append([]etcdObject(nil), objs...)
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Kind != rows[j].Kind {
			return rows[i].Kind < rows[j].Kind
		}
		return rows[i].Name < rows[j].Name
	})

	p := profile{Objects: rows, Total: kindProfile{Kind: totalRow}}
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

// renderObjects lists every profiled object, which is where a single hot
// TaskRun or oversized Pod shows up; the per-Kind summary alone hides it.
func renderObjects(p profile) string {
	// Size the name column from the data. Pod and Event names carry the
	// PipelineRun's own name plus a suffix, so any fixed width wide enough for
	// them is mostly padding, and any narrower one bends the table.
	width := len("NAME")
	for _, o := range p.Objects {
		if len(o.Name) > width {
			width = len(o.Name)
		}
	}

	name := "%-" + strconv.Itoa(width) + "s"
	header := "%-14s " + name + " %10s %14s %18s\n"
	row := "%-14s " + name + " %10d %14d %18d\n"

	var b strings.Builder
	fmt.Fprintf(&b, header, "KIND", "NAME", "REVISIONS", "CURRENT(B)", "EST-WRITE-BYTES(B)")
	for _, o := range p.Objects {
		fmt.Fprintf(&b, row, o.Kind, o.Name, o.Version, o.ValueBytes, o.Version*int64(o.ValueBytes))
	}
	return b.String()
}

// renderTable formats a profile as a fixed-width text table.
func renderTable(p profile) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-14s %6s %10s %14s %18s\n", "KIND", "COUNT", "REVISIONS", "CURRENT(B)", "EST-WRITE-BYTES(B)")
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
