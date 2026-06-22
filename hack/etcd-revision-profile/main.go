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
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
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
	list(namespace, pipelineRun string) ([]profiledRef, error)
}

// etcdGetter returns the etcd footprint of a single storage key.
type etcdGetter interface {
	get(key string) (etcdObject, error)
}

// buildProfile resolves every discovered object to its etcd key, fetches its
// revision footprint, and aggregates the result. Objects whose key is missing
// (e.g. deleted mid-run) are returned as per-object errors and skipped rather
// than aborting the whole profile.
func buildProfile(l objectLister, g etcdGetter, namespace, pipelineRun string) (profile, []error, error) {
	refs, err := l.list(namespace, pipelineRun)
	if err != nil {
		return profile{}, nil, fmt.Errorf("discovering objects: %w", err)
	}
	var objs []etcdObject
	var objErrs []error
	for _, r := range refs {
		key := etcdKeyFor(r.Ref)
		o, err := g.get(key)
		if err != nil {
			objErrs = append(objErrs, fmt.Errorf("%s %s/%s (%s): %w", r.Kind, r.Ref.Namespace, r.Ref.Name, key, err))
			continue
		}
		o.Kind = r.Kind
		o.Namespace = r.Ref.Namespace
		o.Name = r.Ref.Name
		objs = append(objs, o)
	}
	return aggregate(objs), objErrs, nil
}

// commandRunner runs an external command and returns its stdout (a test seam).
type commandRunner func(name string, args ...string) ([]byte, error)

func execRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// kubectlLister discovers the object set with `kubectl`.
type kubectlLister struct {
	bin string
	run commandRunner
}

func (k kubectlLister) names(namespace, resource, labelSelector string) ([]string, error) {
	args := []string{"get", resource, "-n", namespace, "-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}"}
	if labelSelector != "" {
		args = append(args, "-l", labelSelector)
	}
	out, err := k.run(k.bin, args...)
	if err != nil {
		return nil, fmt.Errorf("kubectl get %s: %w", resource, err)
	}
	var names []string
	for _, n := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

func (k kubectlLister) eventNames(namespace, involvedName string) ([]string, error) {
	out, err := k.run(k.bin, "get", "events", "-n", namespace,
		"--field-selector", "involvedObject.name="+involvedName,
		"-o", "jsonpath={range .items[*]}{.metadata.name}{\"\\n\"}{end}")
	if err != nil {
		return nil, fmt.Errorf("kubectl get events for %s: %w", involvedName, err)
	}
	var names []string
	for _, n := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n = strings.TrimSpace(n); n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

func (k kubectlLister) list(namespace, pipelineRun string) ([]profiledRef, error) {
	refs := []profiledRef{{
		Kind: "PipelineRun",
		Ref:  objectRef{Group: "tekton.dev", Resource: "pipelineruns", Namespace: namespace, Name: pipelineRun},
	}}
	involved := []string{pipelineRun}

	taskRuns, err := k.names(namespace, "taskruns.tekton.dev", "tekton.dev/pipelineRun="+pipelineRun)
	if err != nil {
		return nil, err
	}
	for _, tr := range taskRuns {
		refs = append(refs, profiledRef{Kind: "TaskRun", Ref: objectRef{Group: "tekton.dev", Resource: "taskruns", Namespace: namespace, Name: tr}})
		involved = append(involved, tr)
	}

	pods, err := k.names(namespace, "pods", "tekton.dev/pipelineRun="+pipelineRun)
	if err != nil {
		return nil, err
	}
	for _, p := range pods {
		refs = append(refs, profiledRef{Kind: "Pod", Ref: objectRef{Resource: "pods", Namespace: namespace, Name: p}})
		involved = append(involved, p)
	}

	for _, name := range involved {
		events, err := k.eventNames(namespace, name)
		if err != nil {
			return nil, err
		}
		for _, e := range events {
			refs = append(refs, profiledRef{Kind: "Event", Ref: objectRef{Resource: "events", Namespace: namespace, Name: e}})
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

func (e etcdctlGetter) get(key string) (etcdObject, error) {
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
	out, err := e.run(name, args...)
	if err != nil {
		return etcdObject{}, fmt.Errorf("etcdctl get %s: %w", key, err)
	}
	return parseEtcdGetJSON(out)
}

func main() {
	var (
		namespace   = flag.String("n", "default", "namespace of the PipelineRun")
		pipelineRun = flag.String("pipelinerun", "", "name of the PipelineRun to profile (required unless -key is set)")
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

	getter := etcdctlGetter{bin: *etcdctlBin, run: execRunner, sudo: *sudo, endpoints: *endpoints, cacert: *cacert, cert: *cert, key: *keyFile}

	if *probeKey != "" {
		o, err := getter.get(*probeKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("%s\n  revisions (version): %d\n  current value bytes: %d\n", o.Key, o.Version, o.ValueBytes)
		return
	}

	if *pipelineRun == "" {
		fmt.Fprintln(os.Stderr, "error: -pipelinerun is required")
		flag.Usage()
		os.Exit(2)
	}

	lister := kubectlLister{bin: *kubectlBin, run: execRunner}

	p, objErrs, err := buildProfile(lister, getter, *namespace, *pipelineRun)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("etcd revision profile for PipelineRun %s/%s\n\n", *namespace, *pipelineRun)
	fmt.Print(renderTable(p))
	if p.Total.CurrentBytes > 0 {
		fmt.Printf("\nrevision amplification (est-rev-bytes / current-bytes): %.1fx\n",
			float64(p.Total.EstRevisionBytes)/float64(p.Total.CurrentBytes))
	}
	for _, e := range objErrs {
		fmt.Fprintf(os.Stderr, "warning: %v\n", e)
	}
}
