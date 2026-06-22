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

import "testing"

type fakeLister struct {
	refs []profiledRef
	err  error
}

func (f fakeLister) list(string, string) ([]profiledRef, error) { return f.refs, f.err }

type fakeGetter struct {
	byKey map[string]etcdObject
}

func (f fakeGetter) get(key string) (etcdObject, error) {
	o, ok := f.byKey[key]
	if !ok {
		return etcdObject{}, errNotFound
	}
	return o, nil
}

// buildProfile must resolve each discovered object to its etcd key, fetch its
// revision footprint, tag it with Kind, and aggregate the result.
func TestBuildProfile(t *testing.T) {
	refs := []profiledRef{
		{Kind: "PipelineRun", Ref: objectRef{Group: "tekton.dev", Resource: "pipelineruns", Namespace: "ci", Name: "build"}},
		{Kind: "TaskRun", Ref: objectRef{Group: "tekton.dev", Resource: "taskruns", Namespace: "ci", Name: "build-compile"}},
		{Kind: "Pod", Ref: objectRef{Resource: "pods", Namespace: "ci", Name: "build-compile-pod"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build":     {Version: 9, ValueBytes: 4000},
		"/registry/tekton.dev/taskruns/ci/build-compile": {Version: 14, ValueBytes: 68000},
		"/registry/pods/ci/build-compile-pod":            {Version: 8, ValueBytes: 12000},
	}}

	p, objErrs, err := buildProfile(fakeLister{refs: refs}, getter, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if len(objErrs) != 0 {
		t.Fatalf("buildProfile() per-object errors: %v", objErrs)
	}
	if p.Total.Count != 3 {
		t.Errorf("Total.Count = %d, want 3", p.Total.Count)
	}
	if p.Total.TotalRevisions != 31 {
		t.Errorf("Total.TotalRevisions = %d, want 31", p.Total.TotalRevisions)
	}
	// EstRevisionBytes = 9*4000 + 14*68000 + 8*12000 = 36000 + 952000 + 96000 = 1084000
	if p.Total.EstRevisionBytes != 1084000 {
		t.Errorf("Total.EstRevisionBytes = %d, want 1084000", p.Total.EstRevisionBytes)
	}
}

// A missing key (e.g. the object was deleted mid-run) must be collected as a
// per-object error and skipped, not abort the whole profile.
func TestBuildProfile_MissingKeyIsCollected(t *testing.T) {
	refs := []profiledRef{
		{Kind: "PipelineRun", Ref: objectRef{Group: "tekton.dev", Resource: "pipelineruns", Namespace: "ci", Name: "build"}},
		{Kind: "Event", Ref: objectRef{Resource: "events", Namespace: "ci", Name: "gone"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 3, ValueBytes: 4000},
	}}

	p, objErrs, err := buildProfile(fakeLister{refs: refs}, getter, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if len(objErrs) != 1 {
		t.Fatalf("got %d per-object errors, want 1: %v", len(objErrs), objErrs)
	}
	if p.Total.Count != 1 {
		t.Errorf("Total.Count = %d, want 1 (missing object skipped)", p.Total.Count)
	}
}
