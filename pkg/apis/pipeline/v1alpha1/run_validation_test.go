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

package v1alpha1_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"knative.dev/pkg/apis"
)

func TestRun_Invalid(t *testing.T) {
	invalidStatusMessage := "test status message"
	for _, c := range []struct {
		name string
		run  *v1alpha1.Run
		want *apis.FieldError
	}{{
		name: "missing spec",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
		},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "Empty spec",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{},
		},
		want: apis.ErrMissingField("spec"),
	}, {
		name: "Both spec and ref missing",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref:                nil,
				Spec:               nil,
				ServiceAccountName: "test-sa",
			},
		},
		want: apis.ErrMissingOneOf("spec.ref", "spec.spec"),
	}, {
		name: "Both Ref and Spec",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "apiVersion",
					Kind:       "kind",
				},
				Spec: &v1alpha1.EmbeddedRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "apiVersion",
						Kind:       "kind",
					},
				},
			},
		},
		want: apis.ErrMultipleOneOf("spec.ref", "spec.spec"),
	}, {
		name: "missing apiVersion in Ref",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "",
				},
			},
		},
		want: apis.ErrMissingField("spec.ref.apiVersion"),
	}, {
		name: "missing kind in Ref",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "",
				},
			},
		},
		want: apis.ErrMissingField("spec.ref.kind"),
	}, {
		name: "missing apiVersion in Spec",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Spec: &v1alpha1.EmbeddedRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "",
					},
				},
			},
		},
		want: apis.ErrMissingField("spec.spec.apiVersion"),
	}, {
		name: "missing kind in Spec",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Spec: &v1alpha1.EmbeddedRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "apiVersion",
						Kind:       "",
					},
				},
			},
		},
		want: apis.ErrMissingField("spec.spec.kind"),
	}, {
		name: "invalid statusMessage in Spec",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				StatusMessage: v1alpha1.RunSpecStatusMessage(invalidStatusMessage),
			},
		},
		want: apis.ErrInvalidValue(fmt.Sprintf("statusMessage should not be set if status is not set, but it is currently set to %s", invalidStatusMessage), "statusMessage"),
	}, {
		name: "non-unique params",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Params: v1beta1.Params{{
					Name:  "foo",
					Value: *v1beta1.NewStructuredValues("foo"),
				}, {
					Name:  "foo",
					Value: *v1beta1.NewStructuredValues("foo"),
				}},
			},
		},
		want: apis.ErrMultipleOneOf("spec.params[foo].name"),
	}} {
		t.Run(c.name, func(t *testing.T) {
			err := c.run.Validate(context.Background())
			if d := cmp.Diff(err.Error(), c.want.Error()); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}

func TestRun_Valid(t *testing.T) {
	for _, c := range []struct {
		name string
		run  *v1alpha1.Run
	}{{
		name: "no params",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
					Name:       "blah",
				},
			},
		},
	}, {
		name: "unnamed",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
			},
		},
	}, {
		name: "unnamed",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Status:        v1alpha1.RunSpecStatusCancelled,
				StatusMessage: v1alpha1.RunSpecStatusMessage("test status vessage"),
			},
		},
	}, {
		name: "Spec with valid ApiVersion and Kind",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Spec: &v1alpha1.EmbeddedRunSpec{
					TypeMeta: runtime.TypeMeta{
						APIVersion: "apiVersion",
						Kind:       "kind",
					},
				},
			},
		},
	}, {
		name: "unique params",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Params: v1beta1.Params{{
					Name:  "foo",
					Value: *v1beta1.NewStructuredValues("foo"),
				}, {
					Name:  "bar",
					Value: *v1beta1.NewStructuredValues("bar"),
				}},
			},
		},
	}, {
		name: "valid workspace",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
	}} {
		t.Run(c.name, func(t *testing.T) {
			if err := c.run.Validate(context.Background()); err != nil {
				t.Fatalf("validating valid Run: %v", err)
			}
		})
	}
}

func TestRun_Workspaces_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		run     *v1alpha1.Run
		wantErr *apis.FieldError
	}{{
		name: "make sure WorkspaceBinding validation invoked",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name: "workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "",
					},
				}},
			},
		},
		wantErr: apis.ErrMissingField("spec.workspaces[0].persistentvolumeclaim.claimname"),
	}, {
		name: "bind same workspace twice",
		run: &v1alpha1.Run{
			ObjectMeta: metav1.ObjectMeta{
				Name: "temp",
			},
			Spec: v1alpha1.RunSpec{
				Ref: &v1beta1.TaskRef{
					APIVersion: "blah",
					Kind:       "blah",
				},
				Workspaces: []v1beta1.WorkspaceBinding{{
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}, {
					Name:     "workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
		wantErr: apis.ErrMultipleOneOf("spec.workspaces[1].name"),
	}}
	for _, ts := range tests {
		t.Run(ts.name, func(t *testing.T) {
			err := ts.run.Validate(context.Background())
			if err == nil {
				t.Errorf("Expected error for invalid Run but got none")
			} else if d := cmp.Diff(ts.wantErr.Error(), err.Error()); d != "" {
				t.Errorf("Did not get expected error for %q: %s", ts.name, diff.PrintWantGot(d))
			}
		})
	}
}
