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

package pipelinerun

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	testing2 "k8s.io/client-go/testing"
	"knative.dev/pkg/kmeta"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

var workspaceName = "test-workspace"

var testPipelineRun = &v1.PipelineRun{
	TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-pipelinerun",
	},
	Spec: v1.PipelineRunSpec{
		Workspaces: []v1.WorkspaceBinding{{
			Name: workspaceName,
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "myclaim",
			},
		}},
	},
}

// TestCreateAndDeleteOfAffinityAssistant tests to create and delete an Affinity Assistant
// for a given PipelineRun
func TestCreateAndDeleteOfAffinityAssistant(t *testing.T) {
	tests := []struct {
		name                  string
		pr                    *v1.PipelineRun
		expectStatefulSetSpec []*appsv1.StatefulSetSpec
	}{{
		name: "PersistentVolumeClaim Workspace type",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-with-pvc"},
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{{
					Name: "PersistentVolumeClaim Workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myclaim",
					},
				}},
			},
		},
		expectStatefulSetSpec: []*appsv1.StatefulSetSpec{{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "workspace-0",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "myclaim"},
						},
					}},
				},
			},
		}},
	}, {
		name: "VolumeClaimTemplate Workspace type",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-with-volumeClaimTemplate"},
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{{
					Name:                "VolumeClaimTemplate Workspace",
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
				}},
			},
		},
		expectStatefulSetSpec: []*appsv1.StatefulSetSpec{{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "pvc-f0680e1c9c"},
			}},
		}},
	}, {
		name: "VolumeClaimTemplate and PersistentVolumeClaim Workspaces",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-with-volumeClaimTemplate-and-pvc"},
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{{
					Name:                "VolumeClaimTemplate Workspace",
					VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
				}, {
					Name: "PersistentVolumeClaim Workspace",
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "myclaim",
					}},
				},
			},
		},
		expectStatefulSetSpec: []*appsv1.StatefulSetSpec{{
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "pvc-f0680e1c9c"},
			}}}, {
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Volumes: []corev1.Volume{{
						Name: "workspace-0",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "myclaim"},
						},
					}},
				},
			},
		}},
	}, {
		name: "other Workspace type",
		pr: &v1.PipelineRun{
			ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-with-emptyDir"},
			Spec: v1.PipelineRunSpec{
				Workspaces: []v1.WorkspaceBinding{{
					Name:     "EmptyDir Workspace",
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				}},
			},
		},
		expectStatefulSetSpec: nil,
	}}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			ctx, cancel := context.WithCancel(ctx)
			defer cancel()

			c := Reconciler{
				KubeClientSet: fakek8s.NewSimpleClientset(),
			}

			err := c.createOrUpdateAffinityAssistants(ctx, tc.pr.Spec.Workspaces, tc.pr, tc.pr.Namespace)
			if err != nil {
				t.Errorf("unexpected error from createOrUpdateAffinityAssistants: %v", err)
			}

			// validate StatefulSets from Affinity Assistant
			for i, w := range tc.pr.Spec.Workspaces {
				expectAAName := getAffinityAssistantName(w.Name, tc.pr.Name)
				aa, err := c.KubeClientSet.AppsV1().StatefulSets(testPipelineRun.Namespace).Get(ctx, expectAAName, metav1.GetOptions{})
				if tc.expectStatefulSetSpec != nil {
					if err != nil {
						t.Fatalf("unexpected error when retrieving StatefulSet: %v", err)
					}

					podSpecFilter := cmpopts.IgnoreFields(corev1.PodSpec{}, "Containers", "Affinity")
					statefulSetSpecFilter := cmpopts.IgnoreFields(appsv1.StatefulSetSpec{}, "Replicas", "Selector")
					podTemplateSpecFilter := cmpopts.IgnoreFields(corev1.PodTemplateSpec{}, "ObjectMeta")
					if d := cmp.Diff(tc.expectStatefulSetSpec[i], &aa.Spec, statefulSetSpecFilter, podSpecFilter, podTemplateSpecFilter); d != "" {
						t.Errorf("StatefulSetSpec diff: %s", diff.PrintWantGot(d))
					}
				} else if !apierrors.IsNotFound(err) {
					t.Errorf("unexpected error when retrieving StatefulSet which expects nil: %v", err)
				}
			}

			// clean up Affinity Assistant
			c.cleanupAffinityAssistants(ctx, tc.pr)
			if err != nil {
				t.Errorf("unexpected error from cleanupAffinityAssistants: %v", err)
			}
			for _, w := range tc.pr.Spec.Workspaces {
				if w.PersistentVolumeClaim == nil && w.VolumeClaimTemplate == nil {
					continue
				}

				expectAAName := getAffinityAssistantName(w.Name, tc.pr.Name)
				_, err = c.KubeClientSet.AppsV1().StatefulSets(tc.pr.Namespace).Get(ctx, expectAAName, metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected a NotFound response, got: %v", err)
				}
			}
		})
	}
}

// TestCreateAffinityAssistantWhenNodeIsCordoned tests an existing Affinity Assistant can identify the node failure and
// can migrate the affinity assistant pod to a healthy node so that the existing pipelineRun runs to competition
func TestCreateOrUpdateAffinityAssistantWhenNodeIsCordoned(t *testing.T) {
	expectedAffinityAssistantName := getAffinityAssistantName(workspaceName, testPipelineRun.Name)

	aa := []*appsv1.StatefulSet{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   expectedAffinityAssistantName,
			Labels: getStatefulSetLabels(testPipelineRun, expectedAffinityAssistantName),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
	}}

	nodes := []*corev1.Node{{
		ObjectMeta: metav1.ObjectMeta{
			Name: "soon-to-be-cordoned-node",
		},
		Spec: corev1.NodeSpec{
			Unschedulable: true,
		},
	}}

	p := []*corev1.Pod{{
		ObjectMeta: metav1.ObjectMeta{
			Name: expectedAffinityAssistantName + "-0",
		},
		Spec: corev1.PodSpec{
			NodeName: "soon-to-be-cordoned-node",
		},
	}}

	tests := []struct {
		name, verb, resource               string
		data                               Data
		validatePodDeletion, expectedError bool
	}{{
		name: "createOrUpdateAffinityAssistants must ignore missing affinity assistant pod, this could be interim and must not fail the entire pipelineRun",
		data: Data{
			StatefulSets: aa,
			Nodes:        nodes,
		},
	}, {
		name: "createOrUpdateAffinityAssistants must delete an affinity assistant pod since the node on which its scheduled is marked as unschedulable",
		data: Data{
			StatefulSets: aa,
			Nodes:        nodes,
			Pods:         p,
		},
		validatePodDeletion: true,
	}, {
		name: "createOrUpdateAffinityAssistants must catch an error while listing nodes",
		data: Data{
			StatefulSets: aa,
			Nodes:        nodes,
		},
		verb:          "list",
		resource:      "nodes",
		expectedError: true,
	}, {
		name: "createOrUpdateAffinityAssistants must catch an error while getting pods",
		data: Data{
			StatefulSets: aa,
			Nodes:        nodes,
		},
		verb:          "get",
		resource:      "pods",
		expectedError: true,
	}, {
		name: "createOrUpdateAffinityAssistants must catch an error while deleting pods",
		data: Data{
			StatefulSets: aa,
			Nodes:        nodes,
			Pods:         p,
		},
		verb:          "delete",
		resource:      "pods",
		expectedError: true,
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, c, cancel := seedTestData(tt.data)
			defer cancel()

			if tt.resource == "nodes" {
				// introduce a reactor to mock node error
				c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor(tt.verb, tt.resource,
					func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
						return true, &corev1.NodeList{}, errors.New("error listing nodes")
					})
			}
			if tt.resource == "pods" {
				// introduce a reactor to mock pod error
				c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor(tt.verb, tt.resource,
					func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
						return true, &corev1.Pod{}, errors.New("error listing/deleting pod")
					})
			}

			err := c.createOrUpdateAffinityAssistants(ctx, testPipelineRun.Spec.Workspaces, testPipelineRun, testPipelineRun.Namespace)
			if !tt.expectedError && err != nil {
				t.Errorf("expected no error from createOrUpdateAffinityAssistants for the test \"%s\", but got: %v", tt.name, err)
			}
			// the affinity assistant pod must have been deleted when it was running on a cordoned node
			if tt.validatePodDeletion {
				_, err = c.KubeClientSet.CoreV1().Pods(testPipelineRun.Namespace).Get(ctx, expectedAffinityAssistantName+"-0", metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected a NotFound response, got: %v", err)
				}
			}
			if tt.expectedError && err == nil {
				t.Errorf("expected error from createOrUpdateAffinityAssistants, but got no error")
			}
		})
	}
}

func TestPipelineRunPodTemplatesArePropagatedToAffinityAssistant(t *testing.T) {
	prWithCustomPodTemplate := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-with-custom-podtemplate",
		},
		Spec: v1.PipelineRunSpec{
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				PodTemplate: &pod.Template{
					Tolerations: []corev1.Toleration{{
						Key:      "key",
						Operator: "Equal",
						Value:    "value",
						Effect:   "NoSchedule",
					}},
					NodeSelector: map[string]string{
						"disktype": "ssd",
					},
					ImagePullSecrets: []corev1.LocalObjectReference{{
						Name: "reg-creds",
					}},
				},
			},
		},
	}

	stsWithTolerationsAndNodeSelector := affinityAssistantStatefulSet("test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []corev1.PersistentVolumeClaimVolumeSource{}, "nginx", nil)

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.NodeSelector) != 1 {
		t.Errorf("expected a NodeSelector in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("expected ImagePullSecrets in the StatefulSet")
	}
}

func TestDefaultPodTemplatesArePropagatedToAffinityAssistant(t *testing.T) {
	prWithCustomPodTemplate := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-with-custom-podtemplate",
		},
	}

	defaultTpl := &pod.AffinityAssistantTemplate{
		Tolerations: []corev1.Toleration{{
			Key:      "key",
			Operator: "Equal",
			Value:    "value",
			Effect:   "NoSchedule",
		}},
		NodeSelector: map[string]string{
			"disktype": "ssd",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{{
			Name: "reg-creds",
		}},
	}

	stsWithTolerationsAndNodeSelector := affinityAssistantStatefulSet("test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []corev1.PersistentVolumeClaimVolumeSource{}, "nginx", defaultTpl)

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.NodeSelector) != 1 {
		t.Errorf("expected a NodeSelector in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("expected ImagePullSecrets in the StatefulSet")
	}
}

func TestMergedPodTemplatesArePropagatedToAffinityAssistant(t *testing.T) {
	prWithCustomPodTemplate := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-with-custom-podtemplate",
		},
		Spec: v1.PipelineRunSpec{
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				PodTemplate: &pod.Template{
					Tolerations: []corev1.Toleration{{
						Key:      "key",
						Operator: "Equal",
						Value:    "value",
						Effect:   "NoSchedule",
					}},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{Name: "reg-creds"},
						{Name: "alt-creds"},
					},
				}},
		},
	}

	defaultTpl := &pod.AffinityAssistantTemplate{
		NodeSelector: map[string]string{
			"disktype": "ssd",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{{
			Name: "reg-creds",
		}},
	}

	stsWithTolerationsAndNodeSelector := affinityAssistantStatefulSet("test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []corev1.PersistentVolumeClaimVolumeSource{}, "nginx", defaultTpl)

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations from spec in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.NodeSelector) != 1 {
		t.Errorf("expected NodeSelector from defaults in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.ImagePullSecrets) != 2 {
		t.Errorf("expected ImagePullSecrets from spec to overwrite default in the StatefulSet")
	}
}

func TestOnlySelectPodTemplateFieldsArePropagatedToAffinityAssistant(t *testing.T) {
	prWithCustomPodTemplate := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-with-custom-podtemplate",
		},
		Spec: v1.PipelineRunSpec{
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				PodTemplate: &pod.Template{
					Tolerations: []corev1.Toleration{{
						Key:      "key",
						Operator: "Equal",
						Value:    "value",
						Effect:   "NoSchedule",
					}},
					HostAliases: []corev1.HostAlias{{
						IP:        "1.2.3.4",
						Hostnames: []string{"localhost"},
					}},
				}},
		},
	}

	stsWithTolerationsAndNodeSelector := affinityAssistantStatefulSet("test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []corev1.PersistentVolumeClaimVolumeSource{}, "nginx", nil)

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations from spec in the StatefulSet")
	}

	if len(stsWithTolerationsAndNodeSelector.Spec.Template.Spec.HostAliases) != 0 {
		t.Errorf("expected HostAliases to not be passed from pod template")
	}
}

func TestThatTheAffinityAssistantIsWithoutNodeSelectorAndTolerations(t *testing.T) {
	prWithoutCustomPodTemplate := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-without-custom-podtemplate",
		},
		Spec: v1.PipelineRunSpec{},
	}

	stsWithoutTolerationsAndNodeSelector := affinityAssistantStatefulSet("test-assistant", prWithoutCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []corev1.PersistentVolumeClaimVolumeSource{}, "nginx", nil)

	if len(stsWithoutTolerationsAndNodeSelector.Spec.Template.Spec.Tolerations) != 0 {
		t.Errorf("unexpected Tolerations in the StatefulSet")
	}

	if len(stsWithoutTolerationsAndNodeSelector.Spec.Template.Spec.NodeSelector) != 0 {
		t.Errorf("unexpected NodeSelector in the StatefulSet")
	}
}

// TestThatAffinityAssistantNameIsNoLongerThan53 tests that the Affinity Assistant Name
// is no longer than 53 chars. This is a limitation with StatefulSet.
// See https://github.com/kubernetes/kubernetes/issues/64023
// This is because the StatefulSet-controller adds a label with the name of the StatefulSet
// plus 10 chars for a hash. Labels in Kubernetes can not be longer than 63 chars.
// Typical output from the example below is affinity-assistant-0384086f62
func TestThatAffinityAssistantNameIsNoLongerThan53(t *testing.T) {
	affinityAssistantName := getAffinityAssistantName(
		"pipeline-workspace-name-that-is-quite-long",
		"pipelinerun-with-a-long-custom-name")

	if len(affinityAssistantName) > 53 {
		t.Errorf("affinity assistant name can not be longer than 53 chars")
	}
}

func TestCleanupAffinityAssistants_Success(t *testing.T) {
	workspace := v1.WorkspaceBinding{
		Name:                "volumeClaimTemplate workspace",
		VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
	}
	pr := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pipelinerun-volumeclaimtemplate",
		},
		Spec: v1.PipelineRunSpec{
			Workspaces: []v1.WorkspaceBinding{workspace},
		},
	}

	// seed data to create StatefulSets and PVCs
	expectedAffinityAssistantName := getAffinityAssistantName(workspace.Name, pr.Name)
	aa := []*appsv1.StatefulSet{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   expectedAffinityAssistantName,
			Labels: getStatefulSetLabels(pr, expectedAffinityAssistantName),
		},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
	}}

	expectedPVCName := getPersistentVolumeClaimNameWithAffinityAssistant(workspace.Name, pr.Name, workspace, *kmeta.NewControllerRef(pr))
	pvc := []*corev1.PersistentVolumeClaim{{
		ObjectMeta: metav1.ObjectMeta{
			Name: expectedPVCName,
		}},
	}

	data := Data{
		StatefulSets: aa,
		PVCs:         pvc,
	}
	ctx, c, _ := seedTestData(data)

	// call clean up
	err := c.cleanupAffinityAssistants(ctx, pr)
	if err != nil {
		t.Fatalf("unexpected err when clean up Affinity Assistant: %v", err)
	}

	// validate the cleanup result
	_, err = c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Get(ctx, expectedAffinityAssistantName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected a NotFound response of StatefulSet, got: %v", err)
	}
	_, err = c.KubeClientSet.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(ctx, expectedPVCName, metav1.GetOptions{})
	if !apierrors.IsNotFound(err) {
		t.Errorf("expected a NotFound response of PersistentVolumeClaims, got: %v", err)
	}
}

func TestCleanupAffinityAssistants_Failure(t *testing.T) {
	pr := &v1.PipelineRun{
		Spec: v1.PipelineRunSpec{
			Workspaces: []v1.WorkspaceBinding{{
				VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
			}},
		},
	}

	ctx := context.Background()
	c := Reconciler{
		KubeClientSet: fakek8s.NewSimpleClientset(),
	}

	// introduce a reactor to mock delete statefulsets & persistentvolumeclaims errors
	c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor("delete", "statefulsets",
		func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
			return true, &corev1.NodeList{}, errors.New("error deleting statefulsets")
		})
	c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor("delete", "persistentvolumeclaims",
		func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
			return true, &corev1.Pod{}, errors.New("error deleting persistentvolumeclaims")
		})

	expectedErrs := errorutils.NewAggregate([]error{
		errors.New("failed to delete StatefulSet affinity-assistant-e3b0c44298: error deleting statefulsets"),
		errors.New("failed to delete PersistentVolumeClaim pvc-e3b0c44298-affinity-assistant-e3b0c44298-0: error deleting persistentvolumeclaims"),
	})

	errs := c.cleanupAffinityAssistants(ctx, pr)
	if errs == nil {
		t.Fatalf("expecting Affinity Assistant cleanup error but got nil")
	}
	if d := cmp.Diff(expectedErrs.Error(), errs.Error()); d != "" {
		t.Errorf("expected err mismatching: %v", diff.PrintWantGot(d))
	}
}

// TestThatCleanupIsAvoidedIfAssistantIsDisabled tests that
// cleanup of Affinity Assistants is omitted when the
// Affinity Assistant is disabled
func TestThatCleanupIsAvoidedIfAssistantIsDisabled(t *testing.T) {
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
		Data: map[string]string{
			featureFlagDisableAffinityAssistantKey: "true",
		},
	}

	fakeClientSet := fakek8s.NewSimpleClientset(
		configMap,
	)

	c := Reconciler{
		KubeClientSet: fakeClientSet,
		Images:        pipeline.Images{},
	}
	store := config.NewStore(logtesting.TestLogger(t))
	store.OnConfigChanged(configMap)

	_ = c.cleanupAffinityAssistants(store.ToContext(context.Background()), testPipelineRun)

	if len(fakeClientSet.Actions()) != 0 {
		t.Errorf("Expected 0 k8s client requests, did %d request", len(fakeClientSet.Actions()))
	}
}

func TestDisableAffinityAssistant(t *testing.T) {
	for _, tc := range []struct {
		description string
		configMap   *corev1.ConfigMap
		expected    bool
	}{{
		description: "Default behaviour: A missing disable-affinity-assistant flag should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data:       map[string]string{},
		},
		expected: false,
	}, {
		description: "Setting disable-affinity-assistant to false should result in false",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				featureFlagDisableAffinityAssistantKey: "false",
			},
		},
		expected: false,
	}, {
		description: "Setting disable-affinity-assistant to true should result in true",
		configMap: &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: config.GetFeatureFlagsConfigName(), Namespace: system.Namespace()},
			Data: map[string]string{
				featureFlagDisableAffinityAssistantKey: "true",
			},
		},
		expected: true,
	}} {
		t.Run(tc.description, func(t *testing.T) {
			c := Reconciler{
				KubeClientSet: fakek8s.NewSimpleClientset(
					tc.configMap,
				),
				Images: pipeline.Images{},
			}
			store := config.NewStore(logtesting.TestLogger(t))
			store.OnConfigChanged(tc.configMap)
			if result := c.isAffinityAssistantDisabled(store.ToContext(context.Background())); result != tc.expected {
				t.Errorf("Expected %t Received %t", tc.expected, result)
			}
		})
	}
}

func TestGetAssistantAffinityMergedWithPodTemplateAffinity(t *testing.T) {
	assistantPodAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 100,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
				},
			},
			TopologyKey: "kubernetes.io/hostname",
		},
	}

	prWithEmptyAffinityPodTemplate := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr-with-no-podTemplate
`)
	affinityWithAssistantAffinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				assistantPodAffinityTerm,
			},
		},
	}

	prWithPodTemplatePodAffinity := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr-with-podTemplate-podAffinity
spec:
  taskRunTemplate:
    podTemplate:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - podAffinityTerm:
              labelSelector:
                matchLabels:
                  test/label: test
              topologyKey: kubernetes.io/hostname
            weight: 50
          requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                test/label: test
            topologyKey: kubernetes.io/hostname
`)
	affinityWithPodTemplatePodAffinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					Weight: 50,
					PodAffinityTerm: corev1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"test/label": "test",
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
				assistantPodAffinityTerm,
			},
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"test/label": "test",
						},
					},
					TopologyKey: "kubernetes.io/hostname",
				},
			},
		},
	}

	prWithPodTemplateNodeAffinity := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr-with-podTemplate-nodeAffinity
spec:
  taskRunTemplate:
    podTemplate:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/hostname
                operator: NotIn
                values:
                - 192.168.xx.xx
`)
	affinityWithPodTemplateNodeAffinity := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				assistantPodAffinityTerm,
			},
		},
		NodeAffinity: &corev1.NodeAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
				NodeSelectorTerms: []corev1.NodeSelectorTerm{
					{
						MatchExpressions: []corev1.NodeSelectorRequirement{
							{
								Key:      "kubernetes.io/hostname",
								Operator: corev1.NodeSelectorOpNotIn,
								Values: []string{
									"192.168.xx.xx",
								},
							},
						},
					},
				},
			},
		},
	}

	for _, tc := range []struct {
		description string
		pr          *v1.PipelineRun
		expect      *corev1.Affinity
	}{
		{
			description: "podTemplate affinity is empty",
			pr:          prWithEmptyAffinityPodTemplate,
			expect:      affinityWithAssistantAffinity,
		},
		{
			description: "podTemplate with affinity which contains podAntiAffinity",
			pr:          prWithPodTemplatePodAffinity,
			expect:      affinityWithPodTemplatePodAffinity,
		},
		{
			description: "podTemplate with affinity which contains nodeAntiAffinity",
			pr:          prWithPodTemplateNodeAffinity,
			expect:      affinityWithPodTemplateNodeAffinity,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			resultAffinity := getAssistantAffinityMergedWithPodTemplateAffinity(tc.pr)
			if d := cmp.Diff(tc.expect, resultAffinity); d != "" {
				t.Errorf("affinity diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

type Data struct {
	StatefulSets []*appsv1.StatefulSet
	Nodes        []*corev1.Node
	Pods         []*corev1.Pod
	PVCs         []*corev1.PersistentVolumeClaim
}

func seedTestData(d Data) (context.Context, Reconciler, func()) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)

	c := Reconciler{
		KubeClientSet: fakek8s.NewSimpleClientset(),
	}
	for _, s := range d.StatefulSets {
		c.KubeClientSet.AppsV1().StatefulSets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
	}
	for _, n := range d.Nodes {
		c.KubeClientSet.CoreV1().Nodes().Create(ctx, n, metav1.CreateOptions{})
	}
	for _, p := range d.Pods {
		c.KubeClientSet.CoreV1().Pods(p.Namespace).Create(ctx, p, metav1.CreateOptions{})
	}
	for _, pvc := range d.PVCs {
		c.KubeClientSet.CoreV1().PersistentVolumeClaims(pvc.Namespace).Create(ctx, pvc, metav1.CreateOptions{})
	}
	return ctx, c, cancel
}
