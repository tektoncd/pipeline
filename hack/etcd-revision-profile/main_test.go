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
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"
)

const prUID = "9f0c1d2e-0000-4000-8000-00000000pr01"

// fake kubectl: answers each `get` by the resource it asks for. Listing calls
// return the tab-separated columns of listJSONPath.
func fakeKubectl(_ context.Context, _ string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "jsonpath={.metadata.uid}"):
		return []byte(prUID + "\n"), nil
	case strings.Contains(joined, "taskruns.tekton.dev"):
		return []byte("demo-build\ttr-uid-1\tPipelineRun\t" + prUID + "\ndemo-test\ttr-uid-2\tPipelineRun\t" + prUID + "\n"), nil
	case strings.Contains(joined, "get pods"):
		return []byte("demo-build-pod\tpod-uid-1\tTaskRun\ttr-uid-1\n"), nil
	case strings.Contains(joined, "get events"):
		return []byte("ev-1\tev-uid-1\n"), nil
	default:
		return nil, nil
	}
}

func TestKubectlListerList(t *testing.T) {
	lister := kubectlLister{bin: "kubectl", run: fakeKubectl}
	refs, _, err := lister.list(context.Background(), "ci", "demo")
	if err != nil {
		t.Fatalf("list() error: %v", err)
	}

	// 1 PipelineRun + 2 TaskRuns + 1 Pod, and one Event per involved object
	// (the PipelineRun, both TaskRuns and the Pod) = 4 events.
	counts := map[string]int{}
	for _, r := range refs {
		counts[r.Kind]++
	}
	want := map[string]int{kindPipelineRun: 1, kindTaskRun: 2, kindPod: 1, kindEvent: 4}
	for kind, n := range want {
		if counts[kind] != n {
			t.Errorf("Kind %s: got %d refs, want %d (all: %v)", kind, counts[kind], n, refs)
		}
	}

	if key := etcdKeyFor(defaultEtcdPrefix, refs[0].Ref); key != "/registry/tekton.dev/pipelineruns/ci/demo" {
		t.Errorf("PipelineRun resolves to key %q", key)
	}
	if refs[0].UID != prUID {
		t.Errorf("PipelineRun UID = %q, want %q", refs[0].UID, prUID)
	}
}

func TestEtcdctlGetterGet(t *testing.T) {
	key := "/registry/tekton.dev/pipelineruns/ci/demo"
	value := []byte("some-protobuf")

	var gotName string
	var gotArgs []string
	run := func(_ context.Context, name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		raw := fmt.Sprintf(`{"header":{"revision":99},"kvs":[{"key":%q,"version":7,"value":%q}],"count":1}`,
			base64.StdEncoding.EncodeToString([]byte(key)),
			base64.StdEncoding.EncodeToString(value))
		return []byte(raw), nil
	}

	getter := etcdctlGetter{bin: "etcdctl", run: run, sudo: true, endpoints: "https://127.0.0.1:2379", cacert: "ca", cert: "crt", key: "k"}
	obj, err := getter.get(context.Background(), key, 0)
	if err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if obj.Version != 7 {
		t.Errorf("Version = %d, want 7", obj.Version)
	}
	if obj.ValueBytes != len(value) {
		t.Errorf("ValueBytes = %d, want %d", obj.ValueBytes, len(value))
	}

	// with -sudo the etcdctl binary is shifted to be sudo's first argument
	if gotName != "sudo" || len(gotArgs) == 0 || gotArgs[0] != "etcdctl" {
		t.Errorf("got command %q %v, want sudo etcdctl ...", gotName, gotArgs)
	}
	if !strings.Contains(strings.Join(gotArgs, " "), key) {
		t.Errorf("get args do not mention key %q: %v", key, gotArgs)
	}
}

// A PipelineRun that has not created anything yet is a clean result, not a
// discovery problem, so it must not produce notes.
func TestKubectlListerNoChildren(t *testing.T) {
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "jsonpath={.metadata.uid}") {
			return []byte(prUID + "\n"), nil
		}
		return nil, nil
	}

	lister := kubectlLister{bin: "kubectl", run: run}
	refs, notes, err := lister.list(context.Background(), "ci", "demo")
	if err != nil {
		t.Fatalf("list() error: %v", err)
	}
	if len(refs) != 1 {
		t.Errorf("got %d refs, want just the PipelineRun", len(refs))
	}
	if len(notes) != 0 {
		t.Errorf("notes = %v, want none when the run simply has no children", notes)
	}
}

func TestEventsRejectsEmptyUID(t *testing.T) {
	called := false
	run := func(context.Context, string, ...string) ([]byte, error) {
		called = true
		return nil, nil
	}
	if _, err := (kubectlLister{bin: "kubectl", run: run}).events(context.Background(), "ci", ""); err == nil {
		t.Error("events() with an empty UID = nil error, want a refusal")
	}
	if called {
		t.Error("events() ran kubectl with an empty involvedObject.uid selector")
	}
}

// A pinned read has to reach etcdctl as --rev, and a zero must not.
func TestEtcdctlGetterPinsRevision(t *testing.T) {
	var gotArgs []string
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		gotArgs = args
		raw := fmt.Sprintf(`{"header":{"revision":9},"kvs":[{"key":%q,"version":1,"value":""}],"count":1}`,
			base64.StdEncoding.EncodeToString([]byte("/k")))
		return []byte(raw), nil
	}
	getter := etcdctlGetter{bin: "etcdctl", run: run}

	if _, err := getter.get(context.Background(), "/k", 4242); err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if !strings.Contains(strings.Join(gotArgs, " "), "--rev=4242") {
		t.Errorf("pinned get did not pass --rev=4242: %v", gotArgs)
	}

	if _, err := getter.get(context.Background(), "/k", 0); err != nil {
		t.Fatalf("get() error: %v", err)
	}
	if strings.Contains(strings.Join(gotArgs, " "), "--rev") {
		t.Errorf("unpinned get passed a --rev flag: %v", gotArgs)
	}
}

// A TaskRun deleted and recreated under the same name need not carry the labels
// discovery selected on. Rechecking through those same selectors would find
// nothing, read that as "it is gone", and accept the replacement's figures as
// the original's, so the recheck lists without a selector.
func TestKubectlListerRechecksWithoutLabelSelector(t *testing.T) {
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "jsonpath={.metadata.uid}"):
			return []byte(prUID + "\n"), nil
		// The replacement carries no PipelineRun label, so it is only visible
		// to a listing that does not filter on one.
		case strings.Contains(joined, "taskruns.tekton.dev") && !strings.Contains(joined, "-l "):
			return []byte("build\ttr-uid-new\t\t\n"), nil
		default:
			return nil, nil
		}
	}

	refs := []profiledRef{
		{Kind: kindTaskRun, UID: "tr-uid-old", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build"}},
	}
	lister := kubectlLister{bin: "kubectl", run: run}
	_, err := lister.recheck(context.Background(), "ci", "demo", refs, refs, true)
	if err == nil {
		t.Fatal("recheck() = nil error for a same-name replacement with no labels, want an error")
	}
	if !strings.Contains(err.Error(), "tr-uid-new") {
		t.Errorf("error = %v, want it to name the UID now serving that name", err)
	}
}

// Keeping its UID but being re-parented means the figures no longer belong to
// this PipelineRun, so the owner is rechecked too.
func TestKubectlListerRechecksOwnership(t *testing.T) {
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "jsonpath={.metadata.uid}"):
			return []byte(prUID + "\n"), nil
		case strings.Contains(joined, "taskruns.tekton.dev") && !strings.Contains(joined, "-l "):
			return []byte("build\ttr-uid-1\tPipelineRun\tsome-other-pr\n"), nil
		default:
			return nil, nil
		}
	}

	refs := []profiledRef{
		{Kind: kindTaskRun, UID: "tr-uid-1", OwnerKind: kindPipelineRun, OwnerUID: prUID,
			Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build"}},
	}
	lister := kubectlLister{bin: "kubectl", run: run}
	if _, err := lister.recheck(context.Background(), "ci", "demo", refs, refs, true); err == nil {
		t.Fatal("recheck() = nil error for a re-parented TaskRun, want an error")
	}
}

// Membership is decided by the controller owner reference, not by labels. A
// TaskRun another PipelineRun owns, a Pod belonging to it, and the Affinity
// Assistant's StatefulSet-owned Pod are all simply not this run's children,
// however they are labelled.
func TestKubectlListerSelectsByOwnership(t *testing.T) {
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "jsonpath={.metadata.uid}"):
			return []byte(prUID + "\n"), nil
		case strings.Contains(joined, "taskruns.tekton.dev"):
			return []byte("mine\ttr-uid-1\tPipelineRun\t" + prUID + "\n" +
				"theirs\ttr-uid-9\tPipelineRun\tsome-other-pr\n" +
				"standalone\ttr-uid-8\t\t\n"), nil
		case strings.Contains(joined, "get pods"):
			return []byte("mine-pod\tpod-uid-1\tTaskRun\ttr-uid-1\n" +
				"theirs-pod\tpod-uid-9\tTaskRun\ttr-uid-9\n" +
				"affinity-assistant-abc-0\taa-uid\tStatefulSet\tss-uid\n"), nil
		default:
			return nil, nil
		}
	}

	lister := kubectlLister{bin: "kubectl", run: run}
	refs, notes, err := lister.list(context.Background(), "ci", "demo")
	if err != nil {
		t.Fatalf("list() error: %v", err)
	}

	got := map[string]bool{}
	for _, r := range refs {
		got[r.Ref.Name] = true
	}
	if !got["mine"] || !got["mine-pod"] {
		t.Errorf("this PipelineRun's own children are missing: %v", got)
	}
	for _, unwanted := range []string{"theirs", "theirs-pod", "standalone", "affinity-assistant-abc-0"} {
		if got[unwanted] {
			t.Errorf("%q was profiled but this PipelineRun does not own it", unwanted)
		}
	}
	// Objects the run does not own are not anomalies worth reporting.
	if len(notes) != 0 {
		t.Errorf("notes = %v, want none", notes)
	}
}

// Discovery must not filter on labels: a child whose labels were stripped is
// still owned by this PipelineRun and still costs etcd revisions.
func TestKubectlListerFindsChildWithoutLabels(t *testing.T) {
	var selectorUsed bool
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-l ") {
			selectorUsed = true
		}
		switch {
		case strings.Contains(joined, "jsonpath={.metadata.uid}"):
			return []byte(prUID + "\n"), nil
		case strings.Contains(joined, "taskruns.tekton.dev"):
			return []byte("unlabelled\ttr-uid-1\tPipelineRun\t" + prUID + "\n"), nil
		default:
			return nil, nil
		}
	}

	lister := kubectlLister{bin: "kubectl", run: run}
	refs, _, err := lister.list(context.Background(), "ci", "demo")
	if err != nil {
		t.Fatalf("list() error: %v", err)
	}
	if selectorUsed {
		t.Error("discovery filtered on a label selector, which a stripped child would escape")
	}
	found := false
	for _, r := range refs {
		if r.Ref.Name == "unlabelled" {
			found = true
		}
	}
	if !found {
		t.Errorf("the unlabelled but owned TaskRun was not discovered: %v", refs)
	}
}

// A run made of CustomRuns or child PipelineRuns would otherwise profile as a
// tidy single row. Those objects are out of scope rather than unmeasured, so
// they are reported as a note and leave the profile complete.
func TestKubectlListerNotesUnsupportedChildren(t *testing.T) {
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "jsonpath={.metadata.uid}"):
			return []byte(prUID + "\n"), nil
		case strings.Contains(joined, "customruns"):
			return []byte("custom-a\tcr-uid\tPipelineRun\t" + prUID + "\n"), nil
		case strings.Contains(joined, "pipelineruns.tekton.dev"):
			return []byte("child-pr\tcpr-uid\tPipelineRun\t" + prUID + "\n"), nil
		default:
			return nil, nil
		}
	}

	lister := kubectlLister{bin: "kubectl", run: run}
	_, notes, err := lister.list(context.Background(), "ci", "demo")
	if err != nil {
		t.Fatalf("list() error: %v", err)
	}
	joined := strings.Join(notes, " | ")
	for _, want := range []string{"custom-a", "child-pr"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notes do not mention %q: %v", want, notes)
		}
	}
}

// The rule that an unconfirmable disappearance is only reportable when the
// stored value was never checked lives in the real lister, so it is tested
// there rather than in a fake that would just restate it.
func TestKubectlListerRecheckReportsUnconfirmableDisappearance(t *testing.T) {
	// The namespace no longer holds the object under any kind.
	run := func(context.Context, string, ...string) ([]byte, error) { return nil, nil }
	refs := []profiledRef{
		{Kind: kindTaskRun, UID: "tr-uid", Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: "ci", Name: "build"}},
	}
	lister := kubectlLister{bin: "kubectl", run: run}

	res, err := lister.recheck(context.Background(), "ci", "demo", refs, refs, false)
	if err != nil {
		t.Fatalf("recheck() error: %v", err)
	}
	if len(res.Unverified) != 1 {
		t.Errorf("Unverified = %v, want the vanished object reported when its stored value was never checked", res.Unverified)
	}

	// With the stored value verified, the same disappearance is accounted for.
	res, err = lister.recheck(context.Background(), "ci", "demo", refs, refs, true)
	if err != nil {
		t.Fatalf("recheck() error: %v", err)
	}
	if len(res.Unverified) != 0 {
		t.Errorf("Unverified = %v, want none once the stored value confirmed the generation", res.Unverified)
	}
}

// The CustomRun lookup only feeds a note, so a cluster that does not serve the
// kind, or a caller without list rights on it, must still get its profile.
func TestKubectlListerSurvivesUnsupportedLookupFailure(t *testing.T) {
	run := func(_ context.Context, _ string, args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "jsonpath={.metadata.uid}"):
			return []byte(prUID + "\n"), nil
		case strings.Contains(joined, "customruns"):
			return nil, errors.New(`the server doesn't have a resource type "customruns"`)
		default:
			return nil, nil
		}
	}

	lister := kubectlLister{bin: "kubectl", run: run}
	refs, notes, err := lister.list(context.Background(), "ci", "demo")
	if err != nil {
		t.Fatalf("list() failed because an informational lookup did: %v", err)
	}
	if len(refs) == 0 {
		t.Error("no refs returned; the PipelineRun itself should still be profiled")
	}
	if len(notes) != 1 || !strings.Contains(notes[0], "could not check") {
		t.Errorf("notes = %v, want one recording that the check could not run", notes)
	}
}
