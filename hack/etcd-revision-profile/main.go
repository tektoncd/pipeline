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

// Command etcd-revision-profile reports, for a single PipelineRun and the
// TaskRuns, Pods and Events belonging to it, how many times each key has been
// written, how large its current stored value is, and an estimate of the
// serialized bytes written over its life.
//
// It is a profiling helper for operators, not part of the Tekton control plane.
// The pure analysis (key layout, etcdctl JSON parsing, aggregation) lives in
// profile.go and is unit-tested; this file is the thin kubectl/etcdctl shell.
//
// Usage:
//
//	etcd-revision-profile -n <namespace> -pipelinerun <name> [etcd flags]
//
// etcd flags default to a kubeadm control-plane node's local endpoint and
// client certificates; -sudo runs etcdctl via sudo because those keys are
// root-only.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sort"
	"strconv"
	"strings"
)

var errNotFound = errors.New("key not found in etcd")

// listJSONPath prints one tab-separated name, UID, and controller owner kind
// and UID per listed object. The owner is what establishes membership.
const listJSONPath = "jsonpath={range .items[*]}{.metadata.name}{\"\\t\"}{.metadata.uid}" +
	"{\"\\t\"}{.metadata.ownerReferences[?(@.controller==true)].kind}" +
	"{\"\\t\"}{.metadata.ownerReferences[?(@.controller==true)].uid}" +
	"{\"\\t\"}{.involvedObject.uid}{\"\\n\"}{end}"

// profiledRef is one object to profile, tagged with the Kind to group it under
// and the UID its Events refer back to.
type profiledRef struct {
	Kind        string
	UID         string
	OwnerKind   string
	OwnerUID    string
	InvolvedUID string
	Ref         objectRef
}

// objectLister discovers the object set belonging to a PipelineRun, along with
// notes on how it decided what belongs. The PipelineRun itself must come first:
// it anchors the report and its read fixes the revision the rest are read at.
type objectLister interface {
	list(ctx context.Context, namespace, pipelineRun string) ([]profiledRef, []string, error)
	// recheck confirms the API still serves the same generation of every
	// profiled object, so a name reused mid-run cannot be counted as the
	// object discovery found.
	recheck(ctx context.Context, namespace, pipelineRun string, discovered, measured []profiledRef, storedUIDVerified bool) (recheckResult, error)
}

// diagnostics separates what is worth knowing from what is missing. Only the
// latter should make a caller distrust the numbers.
type diagnostics struct {
	Info       []string
	Incomplete []string
}

func (d diagnostics) complete() bool { return len(d.Incomplete) == 0 }

// etcdGetter returns the etcd footprint of a single storage key. A rev of 0
// reads the current value; anything else reads the key as of that store-wide
// revision.
type etcdGetter interface {
	get(ctx context.Context, key string, rev int64) (etcdObject, error)
}

// buildProfile resolves every discovered object to its etcd key, fetches its
// revision footprint, and aggregates the result. It returns a warning for each
// thing that left the profile incomplete: an object deleted before it could be
// read, and a key that no longer holds the object discovery found. Discovery's
// own notes come back separately in Info and do not make the profile
// incomplete. Any other getter error aborts so we never report a misleading
// partial result.
//
// etcd keys are built from names, not UIDs, so an object deleted and recreated
// under the same name between discovery and the read would otherwise be counted
// as the object we set out to profile. verifyUID rejects those by checking the
// stored value still carries the UID discovery saw.
func buildProfile(ctx context.Context, l objectLister, g etcdGetter, prefix, namespace, pipelineRun string, verifyUID bool) (profile, diagnostics, error) {
	refs, info, err := l.list(ctx, namespace, pipelineRun)
	d := diagnostics{Info: info}
	if err != nil {
		return profile{}, d, fmt.Errorf("discovering objects: %w", err)
	}
	if len(refs) == 0 {
		return profile{}, d, errors.New("discovery returned no objects")
	}

	// The PipelineRun anchors the whole report, so read it first and insist on
	// it. If its key is gone or now holds another generation there is nothing
	// left to attribute the children to. Its read also fixes the revision every
	// other key is read at, so the rows all describe one point in time rather
	// than whatever each key happened to hold when it was reached.
	root := refs[0]
	if root.Kind != kindPipelineRun || root.Ref.Name != pipelineRun {
		return profile{}, d, fmt.Errorf("discovery did not put %s %s/%s first; the anchor and the pinned revision would come from %s %s",
			kindPipelineRun, namespace, pipelineRun, root.Kind, root.Ref.Name)
	}
	rootKey := etcdKeyFor(prefix, root.Ref)
	rootObj, err := g.get(ctx, rootKey, 0)
	if err != nil {
		return profile{}, d, fmt.Errorf("reading %s %s (%s): %w", root.Kind, root.Ref.Name, rootKey, err)
	}
	if verifyUID && root.UID != "" && !rootObj.holdsUID(root.UID) {
		return profile{}, d, fmt.Errorf("%s %s no longer holds UID %s, so it was replaced during profiling; if values are encrypted at rest, pass -verify-uid=false",
			root.Kind, rootKey, root.UID)
	}
	rev := rootObj.headerRevision

	objs := []etcdObject{annotate(rootObj, root)}
	// Recheck only what the profile actually counted. A child skipped as
	// incomplete is expected to look replaced, and rechecking it would turn a
	// result -allow-partial exists to accept back into a fatal error.
	measured := []profiledRef{root}
	for _, r := range refs[1:] {
		key := etcdKeyFor(prefix, r.Ref)
		o, err := g.get(ctx, key, rev)
		switch {
		case errors.Is(err, errNotFound):
			d.Incomplete = append(d.Incomplete, fmt.Sprintf("skipped %s %s/%s: no key %s at revision %d; it was deleted, or this resource lives in another etcd cluster",
				r.Kind, r.Ref.Namespace, r.Ref.Name, key, rev))
			continue
		case err != nil:
			return profile{}, d, fmt.Errorf("reading %s %s (%s): %w", r.Kind, r.Ref.Name, key, err)
		}
		if verifyUID && r.UID != "" && !o.holdsUID(r.UID) {
			d.Incomplete = append(d.Incomplete, fmt.Sprintf("skipped %s %s/%s: %s no longer holds UID %s, so it was replaced after discovery; if values are encrypted at rest, pass -verify-uid=false",
				r.Kind, r.Ref.Namespace, r.Ref.Name, key, r.UID))
			continue
		}
		objs = append(objs, annotate(o, r))
		measured = append(measured, r)
	}

	// The stored-value check cannot see through encryption at rest, so confirm
	// through the API that every name still resolves to the generation it was
	// profiled as. Without this, -verify-uid=false would leave the whole report
	// open to objects being replaced while they were being read.
	res, err := l.recheck(ctx, namespace, pipelineRun, refs, measured, verifyUID)
	if err != nil {
		return profile{}, d, err
	}

	// An object whose identity could not be confirmed must not contribute
	// figures that were just declared unattributable. Losing the anchor is
	// worse than losing a row: nothing else in the report can be trusted to
	// belong to this run.
	drop := map[string]bool{}
	for _, u := range res.Unverified {
		if u.Kind == kindPipelineRun && u.Ref.Name == pipelineRun {
			return profile{}, d, fmt.Errorf("%s %s/%s could not be confirmed as the object that was measured, so nothing in the report can be attributed to it",
				u.Kind, namespace, u.Ref.Name)
		}
		drop[u.Kind+"/"+u.Ref.Name] = true
		d.Incomplete = append(d.Incomplete, fmt.Sprintf("not counted: %s %s/%s was read, but its identity could not be confirmed afterwards, so its figures are left out",
			u.Kind, namespace, u.Ref.Name))
	}
	if len(drop) > 0 {
		kept := objs[:0]
		for _, o := range objs {
			if !drop[o.Kind+"/"+o.Name] {
				kept = append(kept, o)
			}
		}
		objs = kept
	}

	// Candidates that turned up only after the reads may or may not have
	// existed at the pinned revision. Asking etcd at that revision is the only
	// way to tell a genuine omission from an object created later.
	for _, a := range res.Appeared {
		key := etcdKeyFor(prefix, a.Ref)
		switch _, err := g.get(ctx, key, rev); {
		case errors.Is(err, errNotFound):
			// Created after the revision, so rightly absent from this profile.
		case err != nil:
			return profile{}, d, fmt.Errorf("checking whether %s %s existed at revision %d: %w", a.Kind, a.Ref.Name, rev, err)
		default:
			d.Incomplete = append(d.Incomplete, fmt.Sprintf("not counted: %s %s/%s belongs to this PipelineRun and existed at revision %d, but discovery listed its children before that revision was pinned",
				a.Kind, namespace, a.Ref.Name, rev))
		}
	}
	p := aggregate(objs)
	p.Revision = rev
	return p, d, nil
}

// annotate copies the identity discovery established onto the read object and
// drops the raw value, which is only needed for the UID check.
func annotate(o etcdObject, r profiledRef) etcdObject {
	o.Kind = r.Kind
	o.Namespace = r.Ref.Namespace
	o.Name = r.Ref.Name
	o.UID = r.UID
	o.value = nil
	return o
}

// commandRunner runs an external command and returns its stdout (a test seam).
type commandRunner func(ctx context.Context, name string, args ...string) ([]byte, error)

// execRunner runs the command, returning stdout. On failure it folds the
// command's stderr into the error so kubectl/etcdctl problems are debuggable.
func execRunner(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if msg := strings.TrimSpace(stderr.String()); msg != "" {
			return out, fmt.Errorf("%w: %s", err, msg)
		}
		return out, err
	}
	return out, nil
}

// splitLines turns newline-separated command output into trimmed, non-empty lines.
func splitLines(out []byte) []string {
	var names []string
	for _, n := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names
}

// kubectlLister discovers the object set with `kubectl`.
type kubectlLister struct {
	bin string
	run commandRunner
}

// objectID is a discovered object's identity and its controller owner. Names
// can be reused by a later object, so the UID pins it to one generation, and
// the owner is what proves which parent it belongs to.
type objectID struct {
	Name      string
	UID       string
	OwnerKind string
	OwnerUID  string
	// InvolvedUID is set for Events: the object the Event is about, which is
	// what ties it to this PipelineRun rather than an owner reference.
	InvolvedUID string
}

// ownedBy reports whether this object is controlled by the given parent.
func (o objectID) ownedBy(kind, uid string) bool {
	return o.OwnerKind == kind && o.OwnerUID == uid
}

func (k kubectlLister) objects(ctx context.Context, namespace, resource string) ([]objectID, error) {
	out, err := k.run(ctx, k.bin, "get", resource, "-n", namespace, "-o", listJSONPath)
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s: %w", resource, err)
	}
	return parseListLines(out), nil
}

// parseListLines reads the tab-separated rows printed by listJSONPath.
func parseListLines(out []byte) []objectID {
	var ids []objectID
	for _, line := range splitLines(out) {
		f := strings.Split(line, "\t")
		name := strings.TrimSpace(f[0])
		if name == "" {
			continue
		}
		id := objectID{Name: name}
		if len(f) > 1 {
			id.UID = strings.TrimSpace(f[1])
		}
		if len(f) > 2 {
			id.OwnerKind = strings.TrimSpace(f[2])
		}
		if len(f) > 3 {
			id.OwnerUID = strings.TrimSpace(f[3])
		}
		if len(f) > 4 {
			id.InvolvedUID = strings.TrimSpace(f[4])
		}
		ids = append(ids, id)
	}
	return ids
}

// recheck re-reads the identities the profile was built from and fails if any
// name now resolves to a different UID. It is the only generation check that
// survives encryption at rest, where the stored bytes carry no readable UID.
//
// A name that has disappeared is not an error: the object was read at the
// pinned revision, so those figures still describe it.
// recheckResult is what confirming the measured objects turned up. Neither
// field is fatal on its own: they say the numbers are short, which is what
// -allow-partial exists to accept.
type recheckResult struct {
	// Appeared are objects this run owns that the profile never read. They are
	// candidates only: discovery ran before the revision was pinned, so some
	// existed at it and were missed, while others were created after it and are
	// rightly absent. Reading each one at the pinned revision tells them apart.
	Appeared []profiledRef
	// Unverified are measured objects that vanished before their identity could
	// be confirmed, with nothing else having established it. Their figures
	// cannot be attributed and must not be counted.
	Unverified []profiledRef
}

func (k kubectlLister) recheck(ctx context.Context, namespace, pipelineRun string, discovered, measured []profiledRef, storedUIDVerified bool) (recheckResult, error) {
	// Listed the same way discovery lists, and for the same reason: labels are
	// mutable and prove nothing about identity, so membership is settled again
	// from the UID and the controller owner.
	byKind := map[string]map[string]objectID{}
	// Sorted so a failure always surfaces for the same resource first and the
	// output can be diffed between runs.
	resources := map[string]string{
		kindPipelineRun: resPipelineRuns + "." + groupTektonDev,
		kindTaskRun:     resTaskRuns + "." + groupTektonDev,
		kindPod:         resPods,
		kindEvent:       resEvents,
	}
	for _, kind := range sortedKeys(resources) {
		resource := resources[kind]
		found, err := k.objects(ctx, namespace, resource)
		if err != nil {
			return recheckResult{}, err
		}
		byName := make(map[string]objectID, len(found))
		for _, o := range found {
			byName[o.Name] = o
		}
		byKind[kind] = byName
	}

	res := recheckResult{}
	var prUID string
	for _, r := range discovered {
		if r.Kind == kindPipelineRun && r.Ref.Name == pipelineRun {
			prUID = r.UID
		}
	}
	for _, r := range measured {
		if r.UID == "" {
			continue
		}
		now, ok := byKind[r.Kind][r.Ref.Name]
		if !ok {
			if !storedUIDVerified {
				// Nothing proved which generation was measured: the stored
				// value check is off, and the name is gone before it could be
				// confirmed here. Objects expire routinely, Events most of all,
				// so this makes the profile short rather than unusable.
				res.Unverified = append(res.Unverified, r)
			}
			// Deleted after it was read, but the stored value confirmed the
			// generation at the pinned revision, so those figures stand.
			continue
		}
		if now.UID != r.UID {
			return recheckResult{}, fmt.Errorf("%s %s/%s now has UID %s, not the %s it was profiled as: it was replaced while being read",
				r.Kind, namespace, r.Ref.Name, now.UID, r.UID)
		}
		if r.OwnerUID != "" && (now.OwnerKind != r.OwnerKind || now.OwnerUID != r.OwnerUID) {
			return recheckResult{}, fmt.Errorf("%s %s/%s is now controlled by %s %s, not the %s it was attributed to",
				r.Kind, namespace, r.Ref.Name, now.OwnerKind, now.OwnerUID, r.OwnerUID)
		}
	}

	// Discovery ran before the revision was pinned, so a child created in
	// between exists at that revision without having been read. These listings
	// are taken after the reads, so they can say which ones those were without
	// costing another call.
	res.Appeared = k.unmeasured(namespace, byKind, discovered, prUID)
	return res, nil
}

// sortedKeys returns a map's keys in order, so iteration does not reorder
// output or decide which of several errors is reported.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// unmeasured names the objects this PipelineRun owns that the profile did not
// read, so a child created between discovery and the pinned read stops being a
// silent omission. Discovered refs are excluded as well as measured ones: an
// object that was found and then skipped is already accounted for, and would
// otherwise be reported twice under two different explanations.
//
// Pod and Event ownership is resolved against every TaskRun this run owns
// now, not only the measured ones, so a TaskRun that appeared late does not
// hide the Pods and Events that came with it.
func (k kubectlLister) unmeasured(namespace string, byKind map[string]map[string]objectID, discovered []profiledRef, prUID string) []profiledRef {
	known := map[string]bool{}
	for _, r := range discovered {
		known[r.Kind+"/"+r.Ref.Name] = true
	}

	trUIDs := map[string]bool{}
	for _, o := range byKind[kindTaskRun] {
		if o.ownedBy(kindPipelineRun, prUID) {
			trUIDs[o.UID] = true
		}
	}
	mine := map[string]bool{prUID: true}
	for uid := range trUIDs {
		mine[uid] = true
	}
	for _, o := range byKind[kindPod] {
		if o.OwnerKind == kindTaskRun && trUIDs[o.OwnerUID] {
			mine[o.UID] = true
		}
	}

	var missed []profiledRef
	for name, o := range byKind[kindTaskRun] {
		if !known[kindTaskRun+"/"+name] && o.ownedBy(kindPipelineRun, prUID) {
			missed = append(missed, profiledRef{Kind: kindTaskRun, UID: o.UID,
				Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: namespace, Name: name}})
		}
	}
	for name, o := range byKind[kindPod] {
		if !known[kindPod+"/"+name] && o.OwnerKind == kindTaskRun && trUIDs[o.OwnerUID] {
			missed = append(missed, profiledRef{Kind: kindPod, UID: o.UID,
				Ref: objectRef{Resource: resPods, Namespace: namespace, Name: name}})
		}
	}
	for name, o := range byKind[kindEvent] {
		if !known[kindEvent+"/"+name] && o.InvolvedUID != "" && mine[o.InvolvedUID] {
			missed = append(missed, profiledRef{Kind: kindEvent, UID: o.UID, InvolvedUID: o.InvolvedUID,
				Ref: objectRef{Resource: resEvents, Namespace: namespace, Name: name}})
		}
	}
	sort.Slice(missed, func(i, j int) bool {
		if missed[i].Kind != missed[j].Kind {
			return missed[i].Kind < missed[j].Kind
		}
		return missed[i].Ref.Name < missed[j].Ref.Name
	})
	return missed
}

// uid reads one object's UID, which anchors the whole profile to the requested
// generation of a name that may have been deleted and recreated.
func (k kubectlLister) uid(ctx context.Context, namespace, resource, name string) (string, error) {
	out, err := k.run(ctx, k.bin, "get", resource, name, "-n", namespace, "-o", "jsonpath={.metadata.uid}")
	if err != nil {
		return "", fmt.Errorf("kubectl get %s %s: %w", resource, name, err)
	}
	uid := strings.TrimSpace(string(out))
	if uid == "" {
		return "", fmt.Errorf("%s %s/%s returned no UID", resource, namespace, name)
	}
	return uid, nil
}

func (k kubectlLister) events(ctx context.Context, namespace, involvedUID string) ([]objectID, error) {
	// An empty UID would select every Event whose involvedObject has none,
	// which on a real cluster means unrelated Node and kubelet Events.
	if involvedUID == "" {
		return nil, errors.New("refusing to list events for an object with no UID")
	}
	out, err := k.run(ctx, k.bin, "get", resEvents, "-n", namespace,
		"--field-selector", "involvedObject.uid="+involvedUID,
		"-o", listJSONPath)
	if err != nil {
		return nil, fmt.Errorf("kubectl get events for uid %s: %w", involvedUID, err)
	}
	return parseListLines(out), nil
}

func (k kubectlLister) list(ctx context.Context, namespace, pipelineRun string) ([]profiledRef, []string, error) {
	prUID, err := k.uid(ctx, namespace, resPipelineRuns+"."+groupTektonDev, pipelineRun)
	if err != nil {
		return nil, nil, err
	}

	pr := profiledRef{Kind: kindPipelineRun, UID: prUID, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: namespace, Name: pipelineRun}}
	refs := []profiledRef{pr}
	involved := []profiledRef{pr}

	taskRuns, pods, err := k.children(ctx, namespace, prUID)
	if err != nil {
		return nil, nil, err
	}
	notes := k.unsupported(ctx, namespace, pipelineRun, prUID)
	for _, tr := range taskRuns {
		r := profiledRef{Kind: kindTaskRun, UID: tr.UID, OwnerKind: tr.OwnerKind, OwnerUID: tr.OwnerUID, Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: namespace, Name: tr.Name}}
		refs = append(refs, r)
		involved = append(involved, r)
	}
	for _, p := range pods {
		r := profiledRef{Kind: kindPod, UID: p.UID, OwnerKind: p.OwnerKind, OwnerUID: p.OwnerUID, Ref: objectRef{Resource: resPods, Namespace: namespace, Name: p.Name}}
		refs = append(refs, r)
		involved = append(involved, r)
	}

	for _, obj := range involved {
		events, err := k.events(ctx, namespace, obj.UID)
		if err != nil {
			return nil, notes, err
		}
		for _, e := range events {
			refs = append(refs, profiledRef{Kind: kindEvent, UID: e.UID, InvolvedUID: obj.UID, Ref: objectRef{Resource: resEvents, Namespace: namespace, Name: e.Name}})
		}
	}
	return refs, notes, nil
}

// children finds the PipelineRun's TaskRuns and their Pods by controller owner
// reference, which carries the parent's UID. Tekton's own reconciler settles
// membership the same way when it rebuilds a run's child references, which is
// how it leaves out TaskRuns a custom task created for itself; it compares the
// first owner reference, while this compares the one marked as the controller
// and its kind.
//
// Labels are deliberately not used to narrow the listing. They are mutable and
// can be copied onto an unrelated object, the UID one only exists from Tekton
// v0.63, and a child whose labels were stripped would be missing from a report
// that still called itself complete. Listing the namespace and keeping what
// this PipelineRun owns costs one call per resource and cannot be fooled that
// way. It also leaves nothing to explain away: an object this run does not own,
// the Affinity Assistant's Pod included, is simply not one of its children.
func (k kubectlLister) children(ctx context.Context, namespace, prUID string) ([]objectID, []objectID, error) {
	allTaskRuns, err := k.objects(ctx, namespace, resTaskRuns+"."+groupTektonDev)
	if err != nil {
		return nil, nil, err
	}
	var taskRuns []objectID
	trUIDs := map[string]bool{}
	for _, tr := range allTaskRuns {
		if tr.ownedBy(kindPipelineRun, prUID) {
			taskRuns = append(taskRuns, tr)
			trUIDs[tr.UID] = true
		}
	}

	allPods, err := k.objects(ctx, namespace, resPods)
	if err != nil {
		return nil, nil, err
	}
	var pods []objectID
	for _, p := range allPods {
		if p.OwnerKind == kindTaskRun && trUIDs[p.OwnerUID] {
			pods = append(pods, p)
		}
	}
	return taskRuns, pods, nil
}

// unsupported names the CustomRuns and child PipelineRuns this run owns, so a
// pipeline built out of them is not reported as a tidy single row. Other owned
// kinds, the Affinity Assistant StatefulSet and workspace PVCs among them, are
// out of scope and not checked for. It is a note rather than a gap in the
// numbers, so nothing here can fail the run.
func (k kubectlLister) unsupported(ctx context.Context, namespace, pipelineRun, prUID string) []string {
	var notes []string
	resources := map[string]string{
		kindCustomRun:   resCustomRuns + "." + groupTektonDev,
		kindPipelineRun: resPipelineRuns + "." + groupTektonDev,
	}
	for _, kind := range sortedKeys(resources) {
		found, err := k.objects(ctx, namespace, resources[kind])
		if err != nil {
			// This only feeds a note. A cluster that does not serve CustomRuns,
			// or a caller without list rights on them, must still get its
			// profile.
			notes = append(notes, fmt.Sprintf("could not check for %ss this PipelineRun owns: %v", kind, err))
			continue
		}
		var owned []string
		for _, o := range found {
			// The PipelineRun being profiled is not one of its own children,
			// and a self-referencing owner reference is accepted by the API.
			if o.Name == pipelineRun && kind == kindPipelineRun {
				continue
			}
			if o.ownedBy(kindPipelineRun, prUID) {
				owned = append(owned, o.Name)
			}
		}
		if len(owned) > 0 {
			sort.Strings(owned)
			notes = append(notes, fmt.Sprintf("this PipelineRun owns %d %s(s) that the helper does not profile: %s",
				len(owned), kind, strings.Join(owned, ", ")))
		}
	}
	return notes
}

// etcdctlGetter fetches keys with `etcdctl ... get <key> -w json`.
type etcdctlGetter struct {
	bin       string
	run       commandRunner
	sudo      bool
	endpoints string
	cacert    string
	cert      string
	key       string
}

func (e etcdctlGetter) get(ctx context.Context, key string, rev int64) (etcdObject, error) {
	name, args := e.bin, []string{
		"--endpoints=" + e.endpoints,
		"--cacert=" + e.cacert,
		"--cert=" + e.cert,
		"--key=" + e.key,
		"get", key, "-w", "json",
	}
	if rev > 0 {
		args = append(args, "--rev="+strconv.FormatInt(rev, 10))
	}
	if e.sudo {
		args = append([]string{name}, args...)
		name = "sudo"
	}
	out, err := e.run(ctx, name, args...)
	if err != nil {
		return etcdObject{}, fmt.Errorf("etcdctl get %s: %w", key, err)
	}
	return parseEtcdGetJSON(out, key)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		namespace        = flag.String("n", "default", "namespace of the PipelineRun")
		pipelineRun      = flag.String("pipelinerun", "", "name of the PipelineRun to profile (required unless -etcd-key is set)")
		probeKey         = flag.String("etcd-key", "", "profile a single raw etcd key (e.g. /registry/minions/<node>) and exit; bypasses kubectl discovery")
		etcdPrefix       = flag.String("etcd-prefix", defaultEtcdPrefix, "apiserver --etcd-prefix, the root under which Kubernetes objects are stored")
		allowPartialFlag = flag.Bool("allow-partial", false, "exit successfully even when some objects could not be measured")
		verifyUID        = flag.Bool("verify-uid", true, "skip objects whose stored value no longer carries the UID found during discovery; turn off when values are encrypted at rest")
		kubectlBin       = flag.String("kubectl", "kubectl", "path to the kubectl binary")
		etcdctlBin       = flag.String("etcdctl", "etcdctl", "path to the etcdctl binary")
		sudo             = flag.Bool("sudo", true, "run etcdctl via sudo (etcd client keys are root-only)")
		endpoints        = flag.String("endpoints", "https://127.0.0.1:2379", "etcd client endpoint")
		cacert           = flag.String("cacert", "/etc/kubernetes/pki/etcd/ca.crt", "etcd CA certificate")
		cert             = flag.String("cert", "/etc/kubernetes/pki/etcd/healthcheck-client.crt", "etcd client certificate")
		keyFile          = flag.String("key", "/etc/kubernetes/pki/etcd/healthcheck-client.key", "etcd client key")
	)
	flag.Parse()

	// Cancel in-flight kubectl/etcdctl calls on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	getter := etcdctlGetter{bin: *etcdctlBin, run: execRunner, sudo: *sudo, endpoints: *endpoints, cacert: *cacert, cert: *cert, key: *keyFile}

	if *probeKey != "" {
		o, err := getter.get(ctx, *probeKey, 0)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n  revisions (version): %d\n  current value bytes: %d\n", o.Key, o.Version, o.ValueBytes)
		return nil
	}

	if *pipelineRun == "" {
		flag.Usage()
		return errors.New("-pipelinerun is required (or use -etcd-key)")
	}

	lister := kubectlLister{bin: *kubectlBin, run: execRunner}

	p, diag, err := buildProfile(ctx, lister, getter, *etcdPrefix, *namespace, *pipelineRun, *verifyUID)
	if err != nil {
		// Discovery notes explain how the failed run was correlated, so print
		// them rather than letting the error swallow them.
		printLines("note", diag.Info)
		printLines("missing", diag.Incomplete)
		return err
	}

	fmt.Printf("etcd revision profile for PipelineRun %s/%s (discovered keys read at revision %d)\n\n", *namespace, *pipelineRun, p.Revision)
	fmt.Print(renderObjects(p))
	fmt.Printf("\n%s", renderTable(p))
	if p.Total.CurrentBytes > 0 {
		fmt.Printf("\nestimated rewrite multiple (est-write-bytes / current-bytes): %.1fx\n",
			float64(p.Total.EstRevisionBytes)/float64(p.Total.CurrentBytes))
	}
	printLines("note", diag.Info)
	if !diag.complete() {
		// Say it on stdout too: a redirected report would otherwise look clean.
		fmt.Printf("\nINCOMPLETE: %d object(s) could not be measured, listed on stderr\n", len(diag.Incomplete))
		printLines("missing", diag.Incomplete)
		if !*allowPartialFlag {
			return fmt.Errorf("profile is incomplete (%d object(s) unmeasured); pass -allow-partial to accept it", len(diag.Incomplete))
		}
	}
	return nil
}

func printLines(label string, lines []string) {
	for _, l := range lines {
		fmt.Fprintf(os.Stderr, "%s: %s\n", label, l)
	}
}
