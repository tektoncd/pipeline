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

package resources_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/build-pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/resources"
	buildv1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPostBuildSteps(t *testing.T) {
	taskrun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-output-steps",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       "",
				APIVersion: "a1",
			},
			Outputs: v1alpha1.TaskRunOutputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					Name: "source-workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "source-git",
					},
					Paths: []string{"test-path"},
				}, {
					Name: "source-git-workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "source-git-2",
					},
					Paths: []string{"test-path-2"},
				}, {
					Name: "non-existent-source-no-workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "source-git-non-existent",
					},
					Paths: []string{"non-existent-path"},
				}},
			},
		},
	}

	b := &buildv1alpha1.Build{
		Spec: buildv1alpha1.BuildSpec{
			Sources: []buildv1alpha1.SourceSpec{{
				Name: "source-git",
			}, {
				Name:       "source-git-2",
				TargetPath: "prev-task",
			}, {
				Name: "source-git-no-step",
			}},
		},
	}

	needPVC := resources.AddAfterSteps(taskrun.Spec, b, "pipelinerun-pvc")

	expectedSteps := []corev1.Container{{
		Name:    "source-mkdir-source-git",
		Image:   "busybox",
		Command: []string{"mkdir"},
		Args:    []string{"-p", "test-path"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}, {
		Name:    "source-copy-source-git",
		Image:   "busybox",
		Command: []string{"cp"},
		Args:    []string{"-r", "/workspace/.", "test-path"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}, {
		Name:    "source-mkdir-source-git-2",
		Image:   "busybox",
		Command: []string{"mkdir"},
		Args:    []string{"-p", "test-path-2"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}, {
		Name:    "source-copy-source-git-2",
		Image:   "busybox",
		Command: []string{"cp"},
		Args:    []string{"-r", "/workspace/prev-task/.", "test-path-2"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}}
	if !needPVC {
		t.Errorf("post build steps need pvc should be true but got %t", needPVC)
	}
	if d := cmp.Diff(b.Spec.Steps, expectedSteps); d != "" {
		t.Fatalf("post build steps mismatch: %s", d)
	}
}

func TestPreBuildSteps(t *testing.T) {
	taskrun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-taskrun-run-input-steps",
			Namespace: "foo",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskRef: &v1alpha1.TaskRef{
				Name:       "",
				APIVersion: "a1",
			},
			Inputs: v1alpha1.TaskRunInputs{
				Resources: []v1alpha1.TaskResourceBinding{{
					Name: "source-workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "source-git",
					},
					Paths: []string{"test-path"},
				}, {
					Name: "source-git-workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "source-another-git",
					},
					Paths: []string{"prev-task-1", "prev-task-2"},
				}, {
					Name: "non-existent-source-no-workspace",
					ResourceRef: v1alpha1.PipelineResourceRef{
						Name: "source-git-non-existent",
					},
					Paths: []string{"non-existent-path"},
				}},
			},
		},
	}
	b := &buildv1alpha1.Build{
		Spec: buildv1alpha1.BuildSpec{
			Sources: []buildv1alpha1.SourceSpec{{
				Name: "source-git",
			}, {
				Name:       "source-another-git",
				TargetPath: "new-workspace",
			}, {
				Name: "source-git-no-step",
			}},
		},
	}

	needPVC := resources.AddBeforeSteps(taskrun.Spec, b, "pipelinerun-pvc")
	expectedSteps := []corev1.Container{{
		Name:    "source-copy-source-another-git-0",
		Image:   "busybox",
		Command: []string{"cp"},
		Args:    []string{"-r", "prev-task-1/.", "/workspace/new-workspace"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}, {
		Name:    "source-copy-source-another-git-1",
		Image:   "busybox",
		Command: []string{"cp"},
		Args:    []string{"-r", "prev-task-2/.", "/workspace/new-workspace"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}, {
		Name:    "source-copy-source-git-0",
		Image:   "busybox",
		Command: []string{"cp"},
		Args:    []string{"-r", "test-path/.", "/workspace"},
		VolumeMounts: []corev1.VolumeMount{{
			Name:      "pipelinerun-pvc",
			MountPath: "/pvc",
		}},
	}}
	if !needPVC {
		t.Errorf("pre build steps need pvc should be true but got %t", needPVC)
	}
	if d := cmp.Diff(b.Spec.Steps, expectedSteps); d != "" {
		t.Fatalf("pre build steps mismatch: %s", d)
	}
}

func Test_PVC_Volume(t *testing.T) {
	expectedVolume := corev1.Volume{
		Name: "test-pvc",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "test-pvc"},
		},
	}
	if d := cmp.Diff(expectedVolume, resources.GetPVCVolume("test-pvc")); d != "" {
		t.Fatalf("PVC volume mismatch: %s", d)
	}
}
