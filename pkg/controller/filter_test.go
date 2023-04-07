/*
Copyright 2020 The Tekton Authors

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

package controller_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	fakecustomruninformer "github.com/tektoncd/pipeline/pkg/client/injection/informers/pipeline/v1beta1/customrun/fake"
	"github.com/tektoncd/pipeline/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	rtesting "knative.dev/pkg/reconciler/testing"
)

const (
	apiVersion  = "example.dev/v0"
	apiVersion2 = "example.dev/v1"
	kind        = "Example"
	kind2       = "SomethingCompletelyDifferent"
)

var trueB = true

func TestFilterCustomRunRef(t *testing.T) {
	for _, c := range []struct {
		desc string
		in   interface{}
		want bool
	}{{
		desc: "not a CustomRun",
		in:   struct{}{},
		want: false,
	}, {
		desc: "nil CustomRun",
		in:   (*v1beta1.CustomRun)(nil),
		want: false,
	}, {
		desc: "nil ref and spec",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomRef:  nil,
				CustomSpec: nil,
			},
		},
		want: false,
	}, {
		desc: "both ref and spec",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: "not-matching",
					Kind:       kind,
				},
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion,
						Kind:       kind,
					},
				},
			},
		},
		want: false,
	}, {
		desc: "CustomRun without matching apiVersion in taskRef",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: "not-matching",
					Kind:       kind,
				},
			},
		},
		want: false,
	}, {
		desc: "CustomRun without matching kind in taskRef",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion,
					Kind:       "not-matching",
				},
			},
		},
		want: false,
	}, {
		desc: "CustomRun with matching apiVersion and kind in taskRef",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind,
				},
			},
		},
		want: true,
	}, {
		desc: "CustomRun with matching apiVersion and kind in taskSpec",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion,
						Kind:       kind,
					},
				},
			},
		},
		want: true,
	}, {
		desc: "CustomRun without matching kind for taskSpec",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion,
						Kind:       "not-matching",
					},
				},
			},
		},
		want: false,
	}, {
		desc: "CustomRun without matching apiVersion for taskSpec",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "not-matching",
						Kind:       kind,
					},
				},
			},
		},
		want: false,
	}, {
		desc: "CustomRun with matching apiVersion and kind and name for taskRef",
		in: &v1beta1.CustomRun{
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind,
					Name:       "some-name",
				},
			},
		},
		want: true,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			got := controller.FilterCustomRunRef(apiVersion, kind)(c.in)
			if got != c.want {
				t.Fatalf("FilterCustomRunRef(%q, %q) got %t, want %t", apiVersion, kind, got, c.want)
			}
		})
	}
}

func TestFilterOwnerCustomRunRef(t *testing.T) {
	for _, c := range []struct {
		desc  string
		in    interface{}
		owner *v1beta1.CustomRun
		want  bool
	}{{
		desc: "Owner is a CustomRun for taskRef that references a matching apiVersion and kind",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-customrun",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-customrun",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind,
				},
			},
		},
		want: true,
	}, {
		desc: "Owner is a CustomRun for taskSpec that references a matching apiVersion and kind",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-customrun",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-customrun",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion,
						Kind:       kind,
					},
				},
			},
		},
		want: true,
	}, {
		desc: "Owner is a CustomRun for taskRef that references a non-matching apiversion",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-other-customrun",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-customrun",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion2, // different apiversion
					Kind:       kind,
				},
			},
		},
		want: false,
	}, {
		desc: "Owner is a CustomRun for taskSpec that references a non-matching apiversion",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-other-customrun",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-customrun",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion2, // different apiVersion than expected
						Kind:       kind,
					},
				},
			},
		},
		want: false,
	}, {
		desc: "Owner is a CustomRun for taskRef that references a non-matching kind",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-other-customrun2",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-customrun2",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind2, // different kind than expected
				},
			},
		},
		want: false,
	}, {
		desc: "Owner is a CustomRun for taskSpec that references a non-matching kind",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-other-customrun2",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-other-customrun2",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion,
						Kind:       kind2, // different kind than expected
					},
				},
			},
		},
		want: false,
	}, {
		desc: "Owner is a CustomRun with a missing ref and spec",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-strange-customrun",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-strange-customrun",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{}, // missing ref (illegal)
		},
		want: false,
	}, {
		desc: "Owner is a CustomRun with both ref and spec with matching apiversion and kind",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1beta1.SchemeGroupVersion.String(),
					Kind:       pipeline.CustomRunControllerName,
					Name:       "some-strange-customrun",
					Controller: &trueB,
				}},
			},
		},
		owner: &v1beta1.CustomRun{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1beta1.SchemeGroupVersion.String(),
				Kind:       pipeline.CustomRunControllerName,
			},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-strange-customrun",
				Namespace: "default",
			},
			Spec: v1beta1.CustomRunSpec{
				CustomRef: &v1beta1.TaskRef{
					APIVersion: apiVersion,
					Kind:       kind,
				},
				CustomSpec: &v1beta1.EmbeddedCustomRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: apiVersion,
						Kind:       kind,
					},
				},
			},
		},
		want: false,
	}, {
		desc: "Owner is not a CustomRun",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun",
				Namespace: "default",
				OwnerReferences: []metav1.OwnerReference{{
					APIVersion: v1alpha1.SchemeGroupVersion.String(),
					Kind:       pipeline.PipelineRunControllerName, // owned by PipelineRun, not Run
					Name:       "some-pipelinerun",
					Controller: &trueB,
				}},
			},
		},
		want: false,
	}, {
		desc: "Object has no owner",
		in: &v1beta1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "some-taskrun-no-owner",
				Namespace: "default",
			},
		},
		want: false,
	}, {
		desc: "input is not a runtime Object",
		in:   struct{}{},
		want: false,
	}} {
		t.Run(c.desc, func(t *testing.T) {
			ctx, _ := rtesting.SetupFakeContext(t)
			customRunInformer := fakecustomruninformer.Get(ctx)
			if c.owner != nil {
				if err := customRunInformer.Informer().GetIndexer().Add(c.owner); err != nil {
					t.Fatal(err)
				}
			}
			got := controller.FilterOwnerCustomRunRef(customRunInformer.Lister(), apiVersion, kind)(c.in)
			if got != c.want {
				t.Fatalf("FilterOwnerCustomRunRef(%q, %q) got %t, want %t", apiVersion, kind, got, c.want)
			}
		})
	}
}
