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
	refs     []profiledRef
	notes    []string
	err      error
	replaced bool // the API now serves a different generation of the name
}

func (f fakeLister) list(context.Context, string, string) ([]profiledRef, []string, error) {
	return f.refs, f.notes, f.err
}

func (f fakeLister) recheck(context.Context, string, string, []profiledRef, []profiledRef, bool) (recheckResult, error) {
	if f.replaced {
		return recheckResult{}, errors.New("TaskRun ci/build-compile now has UID tr-uid-new: it was replaced while being read")
	}
	return recheckResult{}, nil
}

type fakeGetter struct {
	byKey map[string]etcdObject
	err   error // when set, returned for every key to simulate a hard failure
}

func (f fakeGetter) get(_ context.Context, key string, _ int64) (etcdObject, error) {
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

	p, diag, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if !diag.complete() {
		t.Fatalf("buildProfile() incomplete: %v", diag.Incomplete)
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

	p, diag, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if len(diag.Incomplete) != 1 {
		t.Fatalf("got %d skipped, want 1: %v", len(diag.Incomplete), diag.Incomplete)
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

	if _, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true); err == nil {
		t.Fatal("buildProfile() = nil error on a hard getter failure, want an error")
	}
}

// The root PipelineRun anchors the report. If its key now holds a different
// generation there is nothing to attribute the children to, so this must abort
// rather than print a profile built from whatever is left.
func TestBuildProfile_RootRecreatedIsFatal(t *testing.T) {
	const discovered, recreated = "uid-generation-1", "uid-generation-2"
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: discovered, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, UID: "tr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build-compile"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {
			Version: 3, ValueBytes: 100,
			value: []byte(`{"metadata":{"uid":"` + recreated + `"}}`),
		},
		// The child survives, which is what made this look like a success
		// before: a non-zero object count hid the replaced root.
		"/registry/tekton.dev/taskruns/ci/build-compile": {
			Version: 5, ValueBytes: 200, value: []byte("tr-uid"),
		},
	}}

	if _, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true); err == nil {
		t.Fatal("buildProfile() = nil error when the root PipelineRun was replaced, want an error")
	}

	// Encrypted values carry no plaintext UID, so the check has to be defeatable.
	p, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", false)
	if err != nil {
		t.Fatalf("buildProfile() with -verify-uid=false error: %v", err)
	}
	if p.Total.Count != 2 {
		t.Errorf("with verification off: Count = %d, want 2", p.Total.Count)
	}
}

// A child replaced after discovery is skipped and reported, but the run
// continues, since one stale child does not invalidate the anchor.
func TestBuildProfile_RecreatedChildIsSkipped(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, UID: "tr-uid-old", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build-compile"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build":     {Version: 3, ValueBytes: 100, value: []byte("pr-uid")},
		"/registry/tekton.dev/taskruns/ci/build-compile": {Version: 5, ValueBytes: 200, value: []byte("tr-uid-new")},
	}}

	p, diag, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Total.Count != 1 {
		t.Errorf("Total.Count = %d, want 1 (the replaced TaskRun must not be counted)", p.Total.Count)
	}
	if len(diag.Incomplete) != 1 || !strings.Contains(diag.Incomplete[0], "tr-uid-old") {
		t.Errorf("incomplete = %v, want one naming the discovered UID", diag.Incomplete)
	}
}

// Every key after the root is read at the revision the root read was served
// at, so the rows describe one point in time instead of a rolling read.
func TestBuildProfile_ReadsChildrenAtRootRevision(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build-compile"}},
	}
	getter := &revRecordingGetter{fakeGetter: fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build":     {Version: 3, ValueBytes: 100, headerRevision: 4242},
		"/registry/tekton.dev/taskruns/ci/build-compile": {Version: 5, ValueBytes: 200, headerRevision: 4300},
	}}}

	if _, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", false); err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if got := getter.revs["/registry/tekton.dev/pipelineruns/ci/build"]; got != 0 {
		t.Errorf("root was read at rev %d, want 0 (latest)", got)
	}
	if got := getter.revs["/registry/tekton.dev/taskruns/ci/build-compile"]; got != 4242 {
		t.Errorf("child was read at rev %d, want the root's revision 4242", got)
	}
}

type revRecordingGetter struct {
	fakeGetter
	revs map[string]int64
}

func (r *revRecordingGetter) get(ctx context.Context, key string, rev int64) (etcdObject, error) {
	if r.revs == nil {
		r.revs = map[string]int64{}
	}
	r.revs[key] = rev
	return r.fakeGetter.get(ctx, key, rev)
}

// The value a matching object stores must satisfy the check, so a healthy
// profile is not turned into warnings.
func TestBuildProfile_MatchingUIDIsCounted(t *testing.T) {
	const uid = "uid-generation-1"
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: uid, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {
			Version: 3, ValueBytes: 100,
			value: []byte("\x0a\x24" + uid + "trailing-protobuf-bytes"),
		},
	}}

	p, diag, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Total.Count != 1 {
		t.Errorf("Total.Count = %d, want 1", p.Total.Count)
	}
	if len(diag.Incomplete) != 0 {
		t.Errorf("incomplete = %v, want none", diag.Incomplete)
	}
}

// On a cluster that encrypts values at rest the stored bytes carry no readable
// UID, so -verify-uid=false is the documented escape hatch. That must not also
// give up the anchor: if the PipelineRun was replaced while it was being read,
// the API still says so.
func TestBuildProfile_RootReplacedIsCaughtWithoutValueCheck(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 3, ValueBytes: 100, value: []byte("ciphertext")},
	}}

	_, _, err := buildProfile(context.Background(), fakeLister{refs: refs, replaced: true}, getter, defaultEtcdPrefix, "ci", "build", false)
	if err == nil {
		t.Fatal("buildProfile() = nil error when the PipelineRun was replaced, want an error")
	}
	if !strings.Contains(err.Error(), "replaced") {
		t.Errorf("error = %v, want it to say the object was replaced", err)
	}
}

// On an encrypted cluster -verify-uid=false turns off the only check the
// stored bytes can support, so the API recheck has to cover the children too:
// a TaskRun recreated under the same name would otherwise be reported with the
// figures of the object that replaced it.
func TestBuildProfile_EncryptedChildReplacedIsCaught(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, UID: "tr-uid-old", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build-compile"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build":     {Version: 3, ValueBytes: 100, value: []byte("ciphertext")},
		"/registry/tekton.dev/taskruns/ci/build-compile": {Version: 5, ValueBytes: 200, value: []byte("ciphertext")},
	}}

	_, _, err := buildProfile(context.Background(), fakeLister{refs: refs, replaced: true}, getter, defaultEtcdPrefix, "ci", "build", false)
	if err == nil {
		t.Fatal("buildProfile() = nil error when a child was replaced, want an error")
	}
}

// A child skipped as incomplete is expected to look replaced. Rechecking it
// would turn a result -allow-partial exists to accept back into a fatal error,
// so only what the profile counted is rechecked.
func TestBuildProfile_SkippedChildIsNotRechecked(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, UID: "tr-old", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "compile"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 10, value: []byte("pr-uid"), headerRevision: 5},
		"/registry/tekton.dev/taskruns/ci/compile":   {Version: 1, ValueBytes: 10, value: []byte("tr-NEW"), headerRevision: 5},
	}}

	l := &recordingLister{refs: refs}
	p, d, err := buildProfile(context.Background(), l, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Total.Count != 1 || len(d.Incomplete) != 1 {
		t.Fatalf("Count = %d, incomplete = %d, want the replaced child skipped", p.Total.Count, len(d.Incomplete))
	}
	for _, r := range l.rechecked {
		if r.Ref.Name == "compile" {
			t.Errorf("the skipped child was rechecked, which would make -allow-partial useless")
		}
	}
}

// A measured object that vanished before its identity could be confirmed, with
// the stored-value check off, leaves the numbers unproven rather than wrong.
// Objects expire routinely, Events most of all, so this makes the profile short
// and lets -allow-partial accept it instead of discarding the whole run.
func TestBuildProfile_UnverifiableDisappearanceIsIncomplete(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 10, value: []byte("pr-uid"), headerRevision: 5},
	}}

	p, d, err := buildProfile(context.Background(), &recordingLister{refs: refs, gone: true}, getter, defaultEtcdPrefix, "ci", "build", false)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if d.complete() {
		t.Error("an unconfirmable object left the profile looking complete")
	}
	if p.Total.Count != 1 {
		t.Errorf("Total.Count = %d, want the object still counted so -allow-partial can accept it", p.Total.Count)
	}
}

// recordingLister reports what it was asked to recheck, and can pretend every
// name has vanished from the API.
type recordingLister struct {
	refs           []profiledRef
	gone           bool
	unverifiedRoot bool
	appeared       []profiledRef
	rechecked      []profiledRef
}

func (l *recordingLister) list(context.Context, string, string) ([]profiledRef, []string, error) {
	return l.refs, nil, nil
}

func (l *recordingLister) recheck(_ context.Context, _, _ string, _ []profiledRef, measured []profiledRef, storedUIDVerified bool) (recheckResult, error) {
	l.rechecked = measured
	if l.unverifiedRoot {
		return recheckResult{Unverified: []profiledRef{{Kind: kindPipelineRun, UID: "pr", Ref: objectRef{Namespace: "ci", Name: "build"}}}}, nil
	}
	if len(l.appeared) > 0 {
		return recheckResult{Appeared: l.appeared}, nil
	}
	if l.gone && !storedUIDVerified {
		// A child, not the anchor: an unverified root is fatal on its own.
		return recheckResult{Unverified: []profiledRef{{Kind: kindTaskRun, UID: "tr", Ref: objectRef{Namespace: "ci", Name: "ghost"}}}}, nil
	}
	return recheckResult{}, nil
}

// The PipelineRun anchors the report and its read fixes the revision every
// other key is read at, so a lister that does not put it first must not be
// allowed to anchor the profile to some other object.
func TestBuildProfile_RootMustBeThePipelineRun(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindEvent, UID: "ev-uid", Ref: objectRef{Resource: resEvents, Namespace: "ci", Name: "build.17abc"}},
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/events/ci/build.17abc":            {Version: 1, ValueBytes: 10, value: []byte("ev-uid"), headerRevision: 999},
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 10, value: []byte("pr-uid"), headerRevision: 5},
	}}

	if _, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true); err == nil {
		t.Fatal("buildProfile() = nil error when an Event was first, want it to refuse that anchor")
	}
}

// The reported revision is the one the reads were pinned to. It cannot be read
// off a row: rows are sorted for output, and only the PipelineRun is read at
// latest, so a child's header carries the store revision at its own read rather
// than the pinned one.
func TestBuildProfile_ReportsThePinnedRevision(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		// An Event sorts before PipelineRun, so it heads the rendered rows.
		{Kind: kindEvent, UID: "ev-uid", Ref: objectRef{Resource: resEvents, Namespace: "ci", Name: "build.17abc"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 10, value: []byte("pr-uid"), headerRevision: 100},
		"/registry/events/ci/build.17abc":            {Version: 1, ValueBytes: 10, value: []byte("ev-uid"), headerRevision: 999},
	}}

	p, _, err := buildProfile(context.Background(), fakeLister{refs: refs}, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Revision != 100 {
		t.Errorf("Revision = %d, want the PipelineRun's 100", p.Revision)
	}
	if p.Objects[0].Kind == kindPipelineRun {
		t.Fatal("test setup: the Event should sort first, or this proves nothing")
	}
}

// An object whose identity could not be confirmed must not contribute figures
// that were just declared unattributable.
func TestBuildProfile_UnverifiedRowIsNotCounted(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
		{Kind: kindTaskRun, UID: "tr", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "ghost"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 100, value: []byte("pr"), headerRevision: 7},
		"/registry/tekton.dev/taskruns/ci/ghost":     {Version: 9, ValueBytes: 3000, value: []byte("tr"), headerRevision: 7},
	}}

	p, d, err := buildProfile(context.Background(), &recordingLister{refs: refs, gone: true}, getter, defaultEtcdPrefix, "ci", "build", false)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	if p.Total.Count != 1 || p.Total.TotalRevisions != 1 {
		t.Errorf("Total = %d objects / %d revisions, want only the PipelineRun: the unattributable row must be left out",
			p.Total.Count, p.Total.TotalRevisions)
	}
	if d.complete() {
		t.Error("dropping a row left the profile looking complete")
	}
}

// Losing the anchor is worse than losing a row: nothing else can be attributed.
func TestBuildProfile_UnverifiedRootIsFatal(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 100, value: []byte("pr"), headerRevision: 7},
	}}
	l := &recordingLister{refs: refs, unverifiedRoot: true}

	if _, _, err := buildProfile(context.Background(), l, getter, defaultEtcdPrefix, "ci", "build", false); err == nil {
		t.Fatal("buildProfile() = nil error when the anchor could not be confirmed, want an error")
	}
}

// A candidate that turned up after the reads is only a real omission if it
// existed at the pinned revision. One created later is rightly absent.
func TestBuildProfile_AppearedIsProbedAtThePinnedRevision(t *testing.T) {
	refs := []profiledRef{
		{Kind: kindPipelineRun, UID: "pr", Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: "ci", Name: "build"}},
	}
	existed := profiledRef{Kind: kindTaskRun, UID: "t1", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "early"}}
	later := profiledRef{Kind: kindTaskRun, UID: "t2", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "late"}}

	getter := fakeGetter{byKey: map[string]etcdObject{
		"/registry/tekton.dev/pipelineruns/ci/build": {Version: 1, ValueBytes: 100, value: []byte("pr"), headerRevision: 7},
		// "early" is readable at the pinned revision; "late" is absent from the map.
		"/registry/tekton.dev/taskruns/ci/early": {Version: 1, ValueBytes: 10, value: []byte("t1"), headerRevision: 7},
	}}
	l := &recordingLister{refs: refs, appeared: []profiledRef{existed, later}}

	_, d, err := buildProfile(context.Background(), l, getter, defaultEtcdPrefix, "ci", "build", true)
	if err != nil {
		t.Fatalf("buildProfile() error: %v", err)
	}
	joined := strings.Join(d.Incomplete, " | ")
	if !strings.Contains(joined, "early") {
		t.Errorf("an object that existed at the pinned revision was not reported: %v", d.Incomplete)
	}
	if strings.Contains(joined, "late") {
		t.Errorf("an object created after the pinned revision was reported as missing: %v", d.Incomplete)
	}
}
