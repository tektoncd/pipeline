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
	"fmt"
	"strings"
	"testing"
)

// fake kubectl: answers each `get` by the resource it asks for.
func fakeKubectl(_ string, args ...string) ([]byte, error) {
	joined := strings.Join(args, " ")
	switch {
	case strings.Contains(joined, "taskruns.tekton.dev"):
		return []byte("demo-build\ndemo-test\n"), nil
	case strings.Contains(joined, "get pods"):
		return []byte("demo-build-pod\n"), nil
	case strings.Contains(joined, "get events"):
		return []byte("ev-1\n"), nil
	default:
		return nil, nil
	}
}

func TestKubectlListerList(t *testing.T) {
	lister := kubectlLister{bin: "kubectl", run: fakeKubectl}
	refs, err := lister.list("ci", "demo")
	if err != nil {
		t.Fatalf("list() error: %v", err)
	}

	// 1 PipelineRun + 2 TaskRuns + 1 Pod, and one Event per involved object
	// (the PipelineRun, both TaskRuns and the Pod) = 4 events.
	counts := map[string]int{}
	for _, r := range refs {
		counts[r.Kind]++
	}
	want := map[string]int{"PipelineRun": 1, "TaskRun": 2, "Pod": 1, "Event": 4}
	for kind, n := range want {
		if counts[kind] != n {
			t.Errorf("Kind %s: got %d refs, want %d (all: %v)", kind, counts[kind], n, refs)
		}
	}

	if key := etcdKeyFor(refs[0].Ref); key != "/registry/tekton.dev/pipelineruns/ci/demo" {
		t.Errorf("PipelineRun resolves to key %q", key)
	}
}

func TestEtcdctlGetterGet(t *testing.T) {
	key := "/registry/tekton.dev/pipelineruns/ci/demo"
	value := []byte("some-protobuf")

	var gotName string
	var gotArgs []string
	run := func(name string, args ...string) ([]byte, error) {
		gotName, gotArgs = name, args
		raw := fmt.Sprintf(`{"kvs":[{"key":%q,"version":7,"value":%q}],"count":1}`,
			base64.StdEncoding.EncodeToString([]byte(key)),
			base64.StdEncoding.EncodeToString(value))
		return []byte(raw), nil
	}

	getter := etcdctlGetter{bin: "etcdctl", run: run, sudo: true, endpoints: "https://127.0.0.1:2379", cacert: "ca", cert: "crt", key: "k"}
	obj, err := getter.get(key)
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
