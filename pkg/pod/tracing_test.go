/*
Copyright 2025 The Tekton Authors
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
package pod
import (
	"testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	fakek8s "k8s.io/client-go/kubernetes/fake"
)
// TestBuildSpans verifies that Build emits an anchor span and a child span for
// each of the expensive steps of the pod construction pipeline.
func TestBuildSpans(t *testing.T) {
	sr := tracetest.NewSpanRecorder()
	tp := tracesdk.NewTracerProvider(tracesdk.WithSpanProcessor(sr))
	ts := v1.TaskSpec{
		Steps: []v1.Step{{
			Name:    "name",
			Image:   "image",
			Command: []string{"cmd"}, // avoid entrypoint lookup.
		}},
	}
	kubeclient := fakek8s.NewSimpleClientset(
		&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: "default", Namespace: "default"}},
	)
	fakeDisc, _ := kubeclient.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDisc.FakedServerVersion = &version.Info{
		Major: "1",
		Minor: "29",
	}
	tr := &v1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "taskrunName",
			Namespace: "default",
		},
		Spec: v1.TaskRunSpec{TaskSpec: &ts},
	}
	b := Builder{
		Images: pipeline.Images{
			EntrypointImage: "entrypoint-image",
			ShellImage:      "busybox",
		},
		KubeClient:      kubeclient,
		EntrypointCache: fakeCache{},
	}
	ctx, parent := tp.Tracer("test").Start(t.Context(), "parent")
	defer parent.End()
	if _, err := b.Build(ctx, tr, ts); err != nil {
		t.Fatalf("Build() = %v", err)
	}
	ended := sr.Ended()
	byName := make(map[string]tracesdk.ReadOnlySpan, len(ended))
	recorded := make([]string, 0, len(ended))
	for _, s := range ended {
		byName[s.Name()] = s
		recorded = append(recorded, s.Name())
	}
	build, ok := byName["Build"]
	if !ok {
		t.Fatalf("expected span %q in recorded spans, got %v", "Build", recorded)
	}
	for _, want := range []string{"credsInit", "convertScripts", "workingDirInit", "resolveEntrypoints", "orderContainers"} {
		child, ok := byName[want]
		if !ok {
			t.Fatalf("expected span %q in recorded spans, got %v", want, recorded)
		}
		if got, wantParent := child.Parent().SpanID(), build.SpanContext().SpanID(); got != wantParent {
			t.Errorf("span %q parent = %s, want %s (the Build span)", want, got, wantParent)
		}
	}
}
