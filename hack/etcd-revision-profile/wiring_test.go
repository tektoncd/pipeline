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
	"strings"
	"testing"
)

type fakeLister struct {
	refs          []profiledRef
	notes         []string
	err           error
	recheckResult recheckResult
	recheckErr    error
	measured      []profiledRef
}

func (f *fakeLister) list(context.Context, string, string) ([]profiledRef, []string, error) {
	return f.refs, f.notes, f.err
}

func (f *fakeLister) recheck(_ context.Context, _, _ string, _, measured []profiledRef) (recheckResult, error) {
	f.measured = measured
	return f.recheckResult, f.recheckErr
}

type fakeGetter struct {
	byKey map[string]etcdObject
	err   error
	revs  map[string]int64
}

func (f *fakeGetter) get(_ context.Context, key string, rev int64) (etcdObject, error) {
	if f.revs == nil {
		f.revs = map[string]int64{}
	}
	f.revs[key] = rev
	if f.err != nil {
		return etcdObject{}, f.err
	}
	o, ok := f.byKey[key]
	if !ok {
		return etcdObject{}, errNotFound
	}
	return o, nil
}

func pipelineRunRef() profiledRef {
	return profiledRef{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}}
}

func TestBuildProfile(t *testing.T) {
	refs := []profiledRef{
		pipelineRunRef(),
		{Kind: kindTaskRun, UID: "tr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build-compile"}},
		{Kind: kindPod, UID: "pod-uid", Ref: objectRef{Resource: resPods, Namespace: "ci", Name: "build-compile-pod"}},
	}
	getter := &fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build":     {Version: 9, ValueBytes: 4000, headerRevision: 4242},
		"/registry/tekton.dev/taskruns/ci/build-compile": {Version: 14, ValueBytes: 68000},
		"/registry/pods/ci/build-compile-pod":            {Version: 8, ValueBytes: 12000},
	}}

	p, diag, err := buildProfile(t.Context(), &fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if !diag.complete() {
		t.Fatalf("buildProfile() incomplete: %v", diag.Incomplete)
	}
	if p.Total.Count != 3 || p.Total.TotalRevisions != 31 || p.Total.EstRevisionBytes != 1084000 {
		t.Errorf("Total = %#v, want 3 objects, 31 revisions and 1084000 estimated bytes", p.Total)
	}
	if p.Revision != 4242 {
		t.Errorf("Revision = %d, want 4242", p.Revision)
	}
	if got := getter.revs["/registry/tekton.dev/taskruns/ci/build-compile"]; got != 4242 {
		t.Errorf("child read at revision %d, want 4242", got)
	}
}

func TestBuildProfileMissingKeyIsIncomplete(t *testing.T) {
	refs := []profiledRef{
		pipelineRunRef(),
		{Kind: kindEvent, UID: "event-uid", Ref: objectRef{Resource: resEvents, Namespace: "ci", Name: "gone"}},
	}
	getter := &fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 3, ValueBytes: 4000, headerRevision: 7},
	}}
	lister := &fakeLister{refs: refs}

	p, diag, err := buildProfile(t.Context(), lister, getter, defaultEtcdPrefix, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Total.Count != 1 || len(diag.Incomplete) != 1 {
		t.Fatalf("Count = %d, incomplete = %v, want one counted and one missing", p.Total.Count, diag.Incomplete)
	}
	if len(lister.measured) != 1 || lister.measured[0].Kind != kindPipelineRun {
		t.Errorf("rechecked = %v, want only the measured PipelineRun", lister.measured)
	}
}

func TestBuildProfileHardGetterErrorAborts(t *testing.T) {
	getter := &fakeGetter{err: errors.New("context deadline exceeded")}
	if _, _, err := buildProfile(t.Context(), &fakeLister{refs: []profiledRef{pipelineRunRef()}}, getter, defaultEtcdPrefix, "ci", "build"); err == nil {
		t.Fatal("buildProfile() = nil error on a hard getter failure")
	}
}

func TestBuildProfileRejectsWrongAnchor(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindEvent, UID: "event-uid", Ref: objectRef{Resource: resEvents, Namespace: "ci", Name: "build.17abc"}},
		pipelineRunRef(),
	}
	if _, _, err := buildProfile(t.Context(), &fakeLister{refs: refs}, &fakeGetter{}, defaultEtcdPrefix, "ci", "build"); err == nil {
		t.Fatal("buildProfile() = nil error when the PipelineRun was not first")
	}
}

func TestBuildProfileReplacementAborts(t *testing.T) {
	lister := &fakeLister{
		refs:       []profiledRef{pipelineRunRef()},
		recheckErr: errors.New("PipelineRun ci/build now has a different UID: it was replaced while being read"),
	}
	getter := &fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 3, ValueBytes: 100, headerRevision: 7},
	}}

	_, _, err := buildProfile(t.Context(), lister, getter, defaultEtcdPrefix, "ci", "build")
	if err == nil || !strings.Contains(err.Error(), "replaced") {
		t.Fatalf("buildProfile() error = %v, want replacement error", err)
	}
}

func TestBuildProfileDropsUnverifiedChild(t *testing.T) {
	child := profiledRef{Kind: kindTaskRun, UID: "tr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "ghost"}}
	lister := &fakeLister{refs: []profiledRef{pipelineRunRef(), child}, recheckResult: recheckResult{Unverified: []profiledRef{child}}}
	getter := &fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 100, headerRevision: 7},
		"/registry/tekton.dev/taskruns/ci/ghost":     {Version: 9, ValueBytes: 3000},
	}}

	p, diag, err := buildProfile(t.Context(), lister, getter, defaultEtcdPrefix, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Total.Count != 1 || p.Total.TotalRevisions != 1 || diag.complete() {
		t.Errorf("Total = %#v, incomplete = %v; want only the PipelineRun and an incomplete profile", p.Total, diag.Incomplete)
	}
}

func TestBuildProfileUnverifiedRootIsFatal(t *testing.T) {
	root := pipelineRunRef()
	lister := &fakeLister{refs: []profiledRef{root}, recheckResult: recheckResult{Unverified: []profiledRef{root}}}
	getter := &fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 100, headerRevision: 7},
	}}

	if _, _, err := buildProfile(t.Context(), lister, getter, defaultEtcdPrefix, "ci", "build"); err == nil {
		t.Fatal("buildProfile() = nil error when the anchor could not be confirmed")
	}
}

func TestBuildProfileChecksLateObjectsAtPinnedRevision(t *testing.T) {
	early := profiledRef{Kind: kindTaskRun, UID: "t1", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "early"}}
	late := profiledRef{Kind: kindTaskRun, UID: "t2", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "late"}}
	lister := &fakeLister{refs: []profiledRef{pipelineRunRef()}, recheckResult: recheckResult{Appeared: []profiledRef{early, late}}}
	getter := &fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 100, headerRevision: 7},
		"/registry/tekton.dev/taskruns/ci/early":     {Version: 1, ValueBytes: 10},
	}}

	_, diag, err := buildProfile(t.Context(), lister, getter, defaultEtcdPrefix, "ci", "build")
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	joined := strings.Join(diag.Incomplete, " | ")
	if !strings.Contains(joined, "early") || strings.Contains(joined, "late") {
		t.Errorf("incomplete = %v, want only the object present at revision 7", diag.Incomplete)
	}
}
