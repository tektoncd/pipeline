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

// Command etcd-revision-profile reports the etcd MVCC footprint of a single
// PipelineRun: how many revisions and how many bytes its PipelineRun, child
// TaskRuns, their Pods, and associated Events have accumulated in etcd.
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
	"strings"
)

var errNotFound = errors.New("key not found in etcd")

// profiledRef is one object to profile, tagged with the Kind to group it under.
type profiledRef struct {
	Kind string
	Ref  objectRef
}

// objectLister discovers the object set belonging to a PipelineRun.
type objectLister interface {
	list(ctx context.Context, namespace, pipelineRun string) ([]profiledRef, error)
}

// etcdGetter returns the etcd footprint of a single storage key.
type etcdGetter interface {
	get(ctx context.Context, key string) (etcdObject, error)
}

// buildProfile resolves every discovered object to its etcd key, fetches its
// revision footprint, and aggregates the result. A missing key (an object
// deleted mid-run) is collected as a skipped object and the profile continues;
// any other getter error aborts so we never report a misleading partial result.
func buildProfile(ctx context.Context, l objectLister, g etcdGetter, namespace, pipelineRun string) (profile, []error, error) {
	refs, err := l.list(ctx, namespace, pipelineRun)
	if err != nil {
		return profile{}, nil, fmt.Errorf("discovering objects: %w", err)
	}
	var objs []etcdObject
	var skipped []error
	for _, r := range refs {
		key := etcdKeyFor(r.Ref)
		o, err := g.get(ctx, key)
		switch {
		case errors.Is(err, errNotFound):
			skipped = append(skipped, fmt.Errorf("%s %s/%s (%s): %w", r.Kind, r.Ref.Namespace, r.Ref.Name, key, err))
			continue
		case err != nil:
			return profile{}, skipped, fmt.Errorf("reading %s %s (%s): %w", r.Kind, r.Ref.Name, key, err)
		}
		o.Kind = r.Kind
		o.Namespace = r.Ref.Namespace
		o.Name = r.Ref.Name
		objs = append(objs, o)
	}
	return aggregate(objs), skipped, nil
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

// splitLines turns newline-separated command output into a trimmed, non-empty
// slice of names.
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

func (k kubectlLister) names(ctx context.Context, namespace, resource, labelSelector string) ([]string, error) {
	args := []string{"get", resource, "-n", namespace, "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"}
	if labelSelector != "" {
		args = append(args, "-l", labelSelector)
	}
	out, err := k.run(ctx, k.bin, args...)
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s: %w", resource, err)
	}
	return splitLines(out), nil
}

func (k kubectlLister) eventNames(ctx context.Context, namespace, involvedKind, involvedName string) ([]string, error) {
	selector := fmt.Sprintf("involvedObject.kind=%s,involvedObject.name=%s", involvedKind, involvedName)
	out, err := k.run(ctx, k.bin, "get", resEvents, "-n", namespace,
		"--field-selector", selector,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}")
	if err != nil {
		return nil, fmt.Errorf("kubectl get events for %s/%s: %w", involvedKind, involvedName, err)
	}
	return splitLines(out), nil
}

func (k kubectlLister) list(ctx context.Context, namespace, pipelineRun string) ([]profiledRef, error) {
	pr := profiledRef{Kind: kindPipelineRun, Ref: objectRef{Group: groupTektonDev, Resource: resPipelineRuns, Namespace: namespace, Name: pipelineRun}}
	refs := []profiledRef{pr}
	involved := []profiledRef{pr}
	selector := groupTektonDev + "/pipelineRun=" + pipelineRun

	taskRuns, err := k.names(ctx, namespace, resTaskRuns+"."+groupTektonDev, selector)
	if err != nil {
		return nil, err
	}
	for _, tr := range taskRuns {
		r := profiledRef{Kind: kindTaskRun, Ref: objectRef{Group: groupTektonDev, Resource: resTaskRuns, Namespace: namespace, Name: tr}}
		refs = append(refs, r)
		involved = append(involved, r)
	}

	pods, err := k.names(ctx, namespace, resPods, selector)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		r := profiledRef{Kind: kindPod, Ref: objectRef{Resource: resPods, Namespace: namespace, Name: p}}
		refs = append(refs, r)
		involved = append(involved, r)
	}

	for _, obj := range involved {
		events, err := k.eventNames(ctx, namespace, obj.Kind, obj.Ref.Name)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			refs = append(refs, profiledRef{Kind: kindEvent, Ref: objectRef{Resource: resEvents, Namespace: namespace, Name: e}})
		}
	}
	return refs, nil
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

func (e etcdctlGetter) get(ctx context.Context, key string) (etcdObject, error) {
	name, args := e.bin, []string{
		"--endpoints=" + e.endpoints,
		"--cacert=" + e.cacert,
		"--cert=" + e.cert,
		"--key=" + e.key,
		"get", key, "-w", "json",
	}
	if e.sudo {
		args = append([]string{name}, args...)
		name = "sudo"
	}
	out, err := e.run(ctx, name, args...)
	if err != nil {
		return etcdObject{}, fmt.Errorf("etcdctl get %s: %w", key, err)
	}
	return parseEtcdGetJSON(out)
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		namespace   = flag.String("n", "default", "namespace of the PipelineRun")
		pipelineRun = flag.String("pipelinerun", "", "name of the PipelineRun to profile (required unless -etcd-key is set)")
		probeKey    = flag.String("etcd-key", "", "profile a single raw etcd key (e.g. /registry/minions/<node>) and exit; bypasses kubectl discovery")
		kubectlBin  = flag.String("kubectl", "kubectl", "path to the kubectl binary")
		etcdctlBin  = flag.String("etcdctl", "etcdctl", "path to the etcdctl binary")
		sudo        = flag.Bool("sudo", true, "run etcdctl via sudo (etcd client keys are root-only)")
		endpoints   = flag.String("endpoints", "https://127.0.0.1:2379", "etcd client endpoint")
		cacert      = flag.String("cacert", "/etc/kubernetes/pki/etcd/ca.crt", "etcd CA certificate")
		cert        = flag.String("cert", "/etc/kubernetes/pki/etcd/healthcheck-client.crt", "etcd client certificate")
		keyFile     = flag.String("key", "/etc/kubernetes/pki/etcd/healthcheck-client.key", "etcd client key")
	)
	flag.Parse()

	// Cancel in-flight kubectl/etcdctl calls on Ctrl-C.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	getter := etcdctlGetter{bin: *etcdctlBin, run: execRunner, sudo: *sudo, endpoints: *endpoints, cacert: *cacert, cert: *cert, key: *keyFile}

	if *probeKey != "" {
		o, err := getter.get(ctx, *probeKey)
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

	p, skipped, err := buildProfile(ctx, lister, getter, *namespace, *pipelineRun)
	if err != nil {
		return err
	}

	fmt.Printf("etcd revision profile for PipelineRun %s/%s\n\n", *namespace, *pipelineRun)
	fmt.Print(renderTable(p))
	if p.Total.CurrentBytes > 0 {
		fmt.Printf("\nrevision amplification (est-rev-bytes / current-bytes): %.1fx\n",
			float64(p.Total.EstRevisionBytes)/float64(p.Total.CurrentBytes))
	}
	for _, e := range skipped {
		fmt.Fprintf(os.Stderr, "skipped: %v\n", e)
	}
	return nil
}
