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
	"context"
	"errors"
	"testing"
)

type fakeLister struct {
	refs []profiledRef
	err  error
}

func (f fakeLister) list(context.Context, string, string) ([]profiledRef, error) {
	return f.refs, f.err
}

type fakeGetter struct {
	byKey map[string]etcdObject
	err   error // when set, returned for every key to simulate a hard failure
}

func (f fakeGetter) get(_ context.Context, key string) (etcdObject, error) {
	if f.err != nil {
		return etcdObject{}, f.err
	}
	o, ok := f.byKey[key]
	if !ok {
		return etcdObject{}, errNotFound
	}
	return o, nil
}

func TestBuildProfile(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build-compile"}},
		{Kind: kindPod, Ref: objectRef{Resource: resPods, Namespace: "ci", Name: "build-compile-pod"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build":     {Version: 9, ValueBytes: 4000},
		"/registry/tekton.dev/taskruns/ci/build-compile": {Version: 14, ValueBytes: 68000},
		"/registry/pods/ci/build-compile-pod":            {Version: 8, ValueBytes: 12000},
	}}

	p, skipped, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if len(skipped) != 0 {
		t.Fatalf("buildProfile() skipped objects: %v", skipped)
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

// A missing key (object deleted mid-run) is skipped and the profile continues.
func TestBuildProfile_MissingKeyIsSkipped(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindEvent, Ref: objectRef{Resource: resEvents, Namespace: "ci", Name: "gone"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 3, ValueBytes: 4000},
	}}

	p, skipped, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if len(skipped) != 1 {
		t.Fatalf("got %d skipped, want 1: %v", len(skipped), skipped)
	}
	if p.Total.Count != 1 {
		t.Errorf("Total.Count = %d, want 1 (missing object skipped)", p.Total.Count)
	}
}

// A getter error that is not "not found" (bad certs, unreachable endpoint) must
// abort the profile, not silently produce a partial result.
func TestBuildProfile_HardErrorAborts(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	getter := fakeGetter{err: errors.New("context deadline exceeded")}

	if _, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, "ci", "build"); err == nil {
		t.Fatal("buildProfile() = nil error on a hard getter failure, want an error")
	}
}
