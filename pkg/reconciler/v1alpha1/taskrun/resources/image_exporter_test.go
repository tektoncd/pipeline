/*
Copyright 2018 The Knative Authors
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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	fakeclientset "github.com/tektoncd/pipeline/pkg/client/clientset/versioned/fake"
	informers "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	outputpipelineImageResourceLister listers.PipelineResourceLister
)

func outputImageResourceSetup() {
	logger, _ = logging.NewLogger("", "")
	fakeClient := fakeclientset.NewSimpleClientset()
	sharedInfomer := informers.NewSharedInformerFactory(fakeClient, 0)
	pipelineResourceInformer := sharedInfomer.Tekton().V1alpha1().PipelineResources()
	outputpipelineImageResourceLister = pipelineResourceInformer.Lister()

	rs := []*v1alpha1.PipelineResource{{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-image-1",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "image",
			Params: []v1alpha1.Param{{
				Name:  "url",
				Value: "gcr.io/some-image-1",
			}, {
				Name:  "digest",
				Value: "",
			}, {
				Name:  "indexpath",
				Value: "/workspace/source-image-1/index.json",
			}},
		},
	}, {
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-image-2",
			Namespace: "marshmallow",
		},
		Spec: v1alpha1.PipelineResourceSpec{
			Type: "image",
			Params: []v1alpha1.Param{{
				Name:  "url",
				Value: "gcr.io/some-image-2",
			}, {
				Name:  "digest",
				Value: "",
			}, {
				Name:  "indexpath",
				Value: "/workspace/source-image-2/index.json",
			}},
		},
	}}

	for _, r := range rs {
		pipelineResourceInformer.Informer().GetIndexer().Add(r)
	}
}

func TestExportingOutputImageResource(t *testing.T) {
	for _, c := range []struct {
		desc      string
		task      *v1alpha1.Task
		taskRun   *v1alpha1.TaskRun
		wantSteps []corev1.Container
	}{{
		desc: "image resource declared as both input and output",
		task: &v1alpha1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "task1",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskSpec{
				Inputs: &v1alpha1.Inputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-image",
						Type: "image",
					}},
				},
				Outputs: &v1alpha1.Outputs{
					Resources: []v1alpha1.TaskResource{{
						Name: "source-image",
						Type: "image",
					}},
				},
			},
		},
		taskRun: &v1alpha1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-taskrun-run-output-steps",
				Namespace: "marshmallow",
			},
			Spec: v1alpha1.TaskRunSpec{
				Inputs: v1alpha1.TaskRunInputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
				Outputs: v1alpha1.TaskRunOutputs{
					Resources: []v1alpha1.TaskResourceBinding{{
						Name: "source-image",
						ResourceRef: v1alpha1.PipelineResourceRef{
							Name: "source-image-1",
						},
					}},
				},
			},
		},
		wantSteps: []corev1.Container{{
			Name:    "image-digest-exporter",
			Image:   "override-with-imagedigest-exporter-image:latest",
			Command: []string{"/ko-app/imagedigestexporter"},
			Args:    []string{"-images", "[{\"name\":\"source-image-1\",\"type\":\"image\",\"url\":\"gcr.io/some-image-1\",\"digest\":\"\",\"indexpath\":\"/workspace/source-image-1/index.json\"}]"},
		}},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			outputImageResourceSetup()
			err := AddOutputImageDigestExporter(c.taskRun, &c.task.Spec, outputpipelineImageResourceLister, logger)
			if err != nil {
				t.Fatalf("Failed to declare output resources for test %q: error %v", c.desc, err)
			}

			if d := cmp.Diff(c.task.Spec.Steps, c.wantSteps); d != "" {
				t.Fatalf("post build steps mismatch: %s", d)
			}
		})
	}
}
