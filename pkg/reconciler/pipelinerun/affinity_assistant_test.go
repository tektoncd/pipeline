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
	"fmt"
	"testing"

	"knative.dev/pkg/ptr"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	cfgtesting "github.com/tektoncd/pipeline/pkg/apis/config/testing"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/pod"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	aa "github.com/tektoncd/pipeline/pkg/internal/affinityassistant"
	pipelinePod "github.com/tektoncd/pipeline/pkg/pod"
	"github.com/tektoncd/pipeline/pkg/reconciler/volumeclaim"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	"go.uber.org/zap"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	errorutils "k8s.io/apimachinery/pkg/util/errors"
	fakek8s "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/typed/core/v1/fake"
	testing2 "k8s.io/client-go/testing"
	logtesting "knative.dev/pkg/logging/testing"
	"knative.dev/pkg/system"
	_ "knative.dev/pkg/system/testing" // Setup system.Namespace()
)

var (
	podSpecFilter         cmp.Option = cmpopts.IgnoreFields(corev1.PodSpec{}, "Affinity")
	podTemplateSpecFilter cmp.Option = cmpopts.IgnoreFields(corev1.PodTemplateSpec{}, "ObjectMeta")
	podContainerFilter    cmp.Option = cmpopts.IgnoreFields(corev1.Container{}, "Resources", "Args", "VolumeMounts")

	containerConfigWithoutSecurityContext = aa.ContainerConfig{
		Image:              "nginx",
		SetSecurityContext: false,
	}
)

var (
	workspacePVCName                 = "test-workspace-pvc"
	workspaceVolumeClaimTemplateName = "test-workspace-vct"
)

var testPRWithPVC = &v1.PipelineRun{
	TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
	ObjectMeta: metav1.ObjectMeta{
		Name: "test-pipelinerun",
	},
	Spec: v1.PipelineRunSpec{
		Workspaces: []v1.WorkspaceBinding{{
			Name: workspacePVCName,
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "myclaim",
			},
		}},
	},
}

var testPRWithVolumeClaimTemplate = &v1.PipelineRun{
	TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
	ObjectMeta: metav1.ObjectMeta{
		Name: "pipelinerun-with-volumeClaimTemplate",
	},
	Spec: v1.PipelineRunSpec{
		Workspaces: []v1.WorkspaceBinding{{
			Name:                workspaceVolumeClaimTemplateName,
			VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
		}},
	},
}

var testPRWithVolumeClaimTemplateAndPVC = &v1.PipelineRun{
	TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
	ObjectMeta: metav1.ObjectMeta{
		Name: "pipelinerun-with-volumeClaimTemplate-and-pvc",
	},
	Spec: v1.PipelineRunSpec{
		Workspaces: []v1.WorkspaceBinding{
			{
				Name:                workspaceVolumeClaimTemplateName,
				VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
			}, {
				Name: workspacePVCName,
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "myclaim",
				},
			},
		},
	},
}

var testPRWithEmptyDir = &v1.PipelineRun{
	ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-with-emptyDir"},
	Spec: v1.PipelineRunSpec{
		Workspaces: []v1.WorkspaceBinding{{
			Name:     "EmptyDir Workspace",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	},
}

var testPRWithWindowsOs = &v1.PipelineRun{
	ObjectMeta: metav1.ObjectMeta{Name: "pipelinerun-with-windows"},
	Spec: v1.PipelineRunSpec{
		TaskRunTemplate: v1.PipelineTaskRunTemplate{
			PodTemplate: &pod.PodTemplate{
				NodeSelector: map[string]string{pipelinePod.OsSelectorLabel: "windows"},
			},
		},
		Workspaces: []v1.WorkspaceBinding{{
			Name:     "EmptyDir Workspace",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
	},
}

// TestCreateOrUpdateAffinityAssistantsAndPVCsPerPipelineRun tests to create and delete Affinity Assistants and PVCs
// per pipelinerun for a given PipelineRun
func TestCreateOrUpdateAffinityAssistantsAndPVCsPerPipelineRun(t *testing.T) {
	replicas := int32(1)

	tests := []struct {
		name                  string
		pr                    *v1.PipelineRun
		expectStatefulSetSpec *appsv1.StatefulSetSpec
		featureFlags          map[string]string
	}{{
		name: "PersistentVolumeClaim Workspace type",
		pr:   testPRWithPVC,
		expectStatefulSetSpec: &appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithPVC.Name,
					workspace.LabelInstance:      "affinity-assistant-622aca4516",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
					Volumes: []corev1.Volume{{
						Name: "workspace-0",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "myclaim"},
						},
					}},
				},
			},
		},
	}, {
		name: "VolumeClaimTemplate Workspace type",
		pr:   testPRWithVolumeClaimTemplate,
		expectStatefulSetSpec: &appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithVolumeClaimTemplate.Name,
					workspace.LabelInstance:      "affinity-assistant-426b306c50",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "pvc-b9eea16dce"},
			}},
		},
	}, {
		name: "VolumeClaimTemplate and PersistentVolumeClaim Workspaces",
		pr:   testPRWithVolumeClaimTemplateAndPVC,
		expectStatefulSetSpec: &appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithVolumeClaimTemplateAndPVC.Name,
					workspace.LabelInstance:      "affinity-assistant-5bf44db4a8",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{Name: "pvc-b9eea16dce"},
			}},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
					Volumes: []corev1.Volume{{
						Name: "workspace-0",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "myclaim"},
						},
					}},
				},
			},
		},
	}, {
		name: "other Workspace type",
		pr:   testPRWithEmptyDir,
		expectStatefulSetSpec: &appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithEmptyDir.Name,
					workspace.LabelInstance:      "affinity-assistant-c655a0c8a2",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
				},
			},
		},
	}, {
		name: "securityContext feature enabled and os is Windows",
		pr:   testPRWithWindowsOs,
		featureFlags: map[string]string{
			"set-security-context": "true",
		},
		expectStatefulSetSpec: &appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithWindowsOs.Name,
					workspace.LabelInstance:      "affinity-assistant-01cecfbdec",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					NodeSelector: map[string]string{pipelinePod.OsSelectorLabel: "windows"},
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: pipelinePod.WindowsSecurityContext,
					}},
				},
			},
		},
	}, {
		name: "securityContext feature enabled and os is Linux",
		pr:   testPRWithEmptyDir,
		featureFlags: map[string]string{
			"set-security-context": "true",
		},
		expectStatefulSetSpec: &appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithEmptyDir.Name,
					workspace.LabelInstance:      "affinity-assistant-c655a0c8a2",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: pipelinePod.LinuxSecurityContext,
					}},
				},
			},
		},
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			featureFlags := map[string]string{
				"disable-affinity-assistant": "true",
				"coschedule":                 "pipelineruns",
			}

			for k, v := range tc.featureFlags {
				featureFlags[k] = v
			}

			kubeClientSet := fakek8s.NewSimpleClientset()
			ctx := cfgtesting.SetFeatureFlags(context.Background(), t, featureFlags)
			c := Reconciler{
				KubeClientSet: kubeClientSet,
				pvcHandler:    volumeclaim.NewPVCHandler(kubeClientSet, zap.NewExample().Sugar()),
			}

			err := c.createOrUpdateAffinityAssistantsAndPVCs(ctx, tc.pr, aa.AffinityAssistantPerPipelineRun)
			if err != nil {
				t.Errorf("unexpected error from createOrUpdateAffinityAssistantsPerPipelineRun: %v", err)
			}

			// validate StatefulSets from Affinity Assistant
			expectAAName := GetAffinityAssistantName("", tc.pr.Name)
			validateStatefulSetSpec(t, ctx, c, expectAAName, tc.expectStatefulSetSpec)

			// clean up Affinity Assistant
			c.cleanupAffinityAssistantsAndPVCs(ctx, tc.pr)
			if err != nil {
				t.Errorf("unexpected error from cleanupAffinityAssistants: %v", err)
			}
			_, err = c.KubeClientSet.AppsV1().StatefulSets(tc.pr.Namespace).Get(ctx, expectAAName, metav1.GetOptions{})
			if !apierrors.IsNotFound(err) {
				t.Errorf("expected a NotFound response, got: %v", err)
			}
		})
	}
}

// TestCreateOrUpdateAffinityAssistantsAndPVCsPerWorkspaceOrDisabled tests to create and delete Affinity Assistants and PVCs
// per workspace or disabled for a given PipelineRun
func TestCreateOrUpdateAffinityAssistantsAndPVCsPerWorkspaceOrDisabled(t *testing.T) {
	replicas := int32(1)
	tests := []struct {
		name, expectedPVCName string
		pr                    *v1.PipelineRun
		expectStatefulSetSpec []*appsv1.StatefulSetSpec
		aaBehavior            aa.AffinityAssistantBehavior
	}{{
		name:       "PersistentVolumeClaim Workspace type",
		aaBehavior: aa.AffinityAssistantPerWorkspace,
		pr:         testPRWithPVC,
		expectStatefulSetSpec: []*appsv1.StatefulSetSpec{{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithPVC.Name,
					workspace.LabelInstance:      "affinity-assistant-ac9f8fc5ee",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
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
		name:            "VolumeClaimTemplate Workspace type",
		aaBehavior:      aa.AffinityAssistantPerWorkspace,
		pr:              testPRWithVolumeClaimTemplate,
		expectedPVCName: "pvc-b9eea16dce",
		expectStatefulSetSpec: []*appsv1.StatefulSetSpec{{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithVolumeClaimTemplate.Name,
					workspace.LabelInstance:      "affinity-assistant-4cf1a1c468",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
					Volumes: []corev1.Volume{{
						Name: "workspace-0",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-b9eea16dce"},
						},
					}},
				},
			},
		}},
	}, {
		name:            "VolumeClaimTemplate Workspace type - AA disabled",
		aaBehavior:      aa.AffinityAssistantDisabled,
		pr:              testPRWithVolumeClaimTemplate,
		expectedPVCName: "pvc-b9eea16dce",
	}, {
		name:            "VolumeClaimTemplate and PersistentVolumeClaim Workspaces",
		aaBehavior:      aa.AffinityAssistantPerWorkspace,
		pr:              testPRWithVolumeClaimTemplateAndPVC,
		expectedPVCName: "pvc-b9eea16dce",
		expectStatefulSetSpec: []*appsv1.StatefulSetSpec{{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithVolumeClaimTemplateAndPVC.Name,
					workspace.LabelInstance:      "affinity-assistant-6c87e714a0",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
					Volumes: []corev1.Volume{{
						Name: "workspace-0",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: "pvc-b9eea16dce"},
						},
					}},
				},
			},
		}, {
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					pipeline.PipelineRunLabelKey: testPRWithVolumeClaimTemplateAndPVC.Name,
					workspace.LabelInstance:      "affinity-assistant-6399c93362",
					workspace.LabelComponent:     workspace.ComponentNameAffinityAssistant,
				},
			},
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "affinity-assistant",
						SecurityContext: &corev1.SecurityContext{},
					}},
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
		name:                  "other Workspace type",
		aaBehavior:            aa.AffinityAssistantPerWorkspace,
		pr:                    testPRWithEmptyDir,
		expectStatefulSetSpec: nil,
	}}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			kubeClientSet := fakek8s.NewSimpleClientset()
			c := Reconciler{
				KubeClientSet: kubeClientSet,
				pvcHandler:    volumeclaim.NewPVCHandler(kubeClientSet, zap.NewExample().Sugar()),
			}

			err := c.createOrUpdateAffinityAssistantsAndPVCs(ctx, tc.pr, tc.aaBehavior)
			if err != nil {
				t.Fatalf("unexpected error from createOrUpdateAffinityAssistantsAndPVCs: %v", err)
			}

			// validate StatefulSets from Affinity Assistant
			for i, w := range tc.pr.Spec.Workspaces {
				if tc.expectStatefulSetSpec != nil {
					expectAAName := GetAffinityAssistantName(w.Name, tc.pr.Name)
					validateStatefulSetSpec(t, ctx, c, expectAAName, tc.expectStatefulSetSpec[i])
				}
			}

			// validate PVCs from VolumeClaimTemplate
			if tc.expectedPVCName != "" {
				_, err = c.KubeClientSet.CoreV1().PersistentVolumeClaims("").Get(ctx, tc.expectedPVCName, metav1.GetOptions{})
				if err != nil {
					t.Errorf("unexpected error when retrieving PVC: %v", err)
				}
			}

			// clean up Affinity Assistant
			c.cleanupAffinityAssistantsAndPVCs(ctx, tc.pr)
			if err != nil {
				t.Errorf("unexpected error from cleanupAffinityAssistantsAndPVCs: %v", err)
			}
			for _, w := range tc.pr.Spec.Workspaces {
				if w.PersistentVolumeClaim == nil && w.VolumeClaimTemplate == nil {
					continue
				}

				expectAAName := GetAffinityAssistantName(w.Name, tc.pr.Name)
				_, err = c.KubeClientSet.AppsV1().StatefulSets(tc.pr.Namespace).Get(ctx, expectAAName, metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected a NotFound response, got: %v", err)
				}
			}
		})
	}
}

func TestCreateOrUpdateAffinityAssistantsAndPVCs_Failure(t *testing.T) {
	testCases := []struct {
		name, failureType string
		aaBehavior        aa.AffinityAssistantBehavior
		expectedErr       error
	}{{
		name:        "affinity assistant creation failed - per workspace",
		failureType: "statefulset",
		aaBehavior:  aa.AffinityAssistantPerWorkspace,
		expectedErr: fmt.Errorf("%w: [failed to create StatefulSet affinity-assistant-4cf1a1c468: error creating statefulsets]", ErrAffinityAssistantCreationFailed),
	}, {
		name:        "affinity assistant creation failed - per pipelinerun",
		failureType: "statefulset",
		aaBehavior:  aa.AffinityAssistantPerPipelineRun,
		expectedErr: fmt.Errorf("%w: [failed to create StatefulSet affinity-assistant-426b306c50: error creating statefulsets]", ErrAffinityAssistantCreationFailed),
	}, {
		name:        "pvc creation failed - per workspace",
		failureType: "pvc",
		aaBehavior:  aa.AffinityAssistantPerWorkspace,
		expectedErr: fmt.Errorf("%w: failed to create PVC pvc-b9eea16dce: error creating persistentvolumeclaims", ErrPvcCreationFailed),
	}, {
		name:        "pvc creation failed - disabled",
		failureType: "pvc",
		aaBehavior:  aa.AffinityAssistantDisabled,
		expectedErr: fmt.Errorf("%w: failed to create PVC pvc-b9eea16dce: error creating persistentvolumeclaims", ErrPvcCreationFailed),
	}}

	for _, tc := range testCases {
		ctx := context.Background()
		kubeClientSet := fakek8s.NewSimpleClientset()
		c := Reconciler{
			KubeClientSet: kubeClientSet,
			pvcHandler:    volumeclaim.NewPVCHandler(kubeClientSet, zap.NewExample().Sugar()),
		}

		switch tc.failureType {
		case "pvc":
			c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor("create", "persistentvolumeclaims",
				func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &corev1.PersistentVolumeClaim{}, errors.New("error creating persistentvolumeclaims")
				})
		case "statefulset":
			c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor("create", "statefulsets",
				func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
					return true, &appsv1.StatefulSet{}, errors.New("error creating statefulsets")
				})
		}

		err := c.createOrUpdateAffinityAssistantsAndPVCs(ctx, testPRWithVolumeClaimTemplate, tc.aaBehavior)

		if err == nil {
			t.Errorf("expect error from createOrUpdateAffinityAssistantsAndPVCs but got nil")
		}

		switch tc.failureType {
		case "pvc":
			if !errors.Is(err, ErrPvcCreationFailed) {
				t.Errorf("expected err type mismatching, expecting %v but got: %v", ErrPvcCreationFailed, err)
			}
		case "statefulset":
			if !errors.Is(err, ErrAffinityAssistantCreationFailed) {
				t.Errorf("expected err type mismatching, expecting %v but got: %v", ErrAffinityAssistantCreationFailed, err)
			}
		}
		if d := cmp.Diff(tc.expectedErr.Error(), err.Error()); d != "" {
			t.Errorf("expected err mismatching: %v", diff.PrintWantGot(d))
		}
	}
}

// TestCreateOrUpdateAffinityAssistantWhenNodeIsCordoned tests an existing Affinity Assistant can identify the node failure and
// can migrate the affinity assistant pod to a healthy node so that the existing pipelineRun runs to compleition
func TestCreateOrUpdateAffinityAssistantWhenNodeIsCordoned(t *testing.T) {
	expectedAffinityAssistantName := GetAffinityAssistantName(workspacePVCName, testPRWithPVC.Name)

	ss := []*appsv1.StatefulSet{{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   expectedAffinityAssistantName,
			Labels: getStatefulSetLabels(testPRWithPVC, expectedAffinityAssistantName),
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
		name: "createOrUpdateAffinityAssistantsPerWorkspace must ignore missing affinity assistant pod, this could be interim and must not fail the entire pipelineRun",
		data: Data{
			StatefulSets: ss,
			Nodes:        nodes,
		},
	}, {
		name: "createOrUpdateAffinityAssistantsPerWorkspace must delete an affinity assistant pod since the node on which its scheduled is marked as unschedulable",
		data: Data{
			StatefulSets: ss,
			Nodes:        nodes,
			Pods:         p,
		},
		validatePodDeletion: true,
	}, {
		name: "createOrUpdateAffinityAssistantsPerWorkspace must catch an error while listing nodes",
		data: Data{
			StatefulSets: ss,
			Nodes:        nodes,
		},
		verb:          "list",
		resource:      "nodes",
		expectedError: true,
	}, {
		name: "createOrUpdateAffinityAssistantsPerWorkspace must catch an error while getting pods",
		data: Data{
			StatefulSets: ss,
			Nodes:        nodes,
		},
		verb:          "get",
		resource:      "pods",
		expectedError: true,
	}, {
		name: "createOrUpdateAffinityAssistantsPerWorkspace must catch an error while deleting pods",
		data: Data{
			StatefulSets: ss,
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
			err := c.createOrUpdateAffinityAssistantsAndPVCs(ctx, testPRWithPVC, aa.AffinityAssistantPerWorkspace)
			if !tt.expectedError && err != nil {
				t.Errorf("expected no error from createOrUpdateAffinityAssistantsPerWorkspace for the test \"%s\", but got: %v", tt.name, err)
			}
			// the affinity assistant pod must have been deleted when it was running on a cordoned node
			if tt.validatePodDeletion {
				_, err = c.KubeClientSet.CoreV1().Pods(testPRWithPVC.Namespace).Get(ctx, expectedAffinityAssistantName+"-0", metav1.GetOptions{})
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected a NotFound response, got: %v", err)
				}
			}
			if tt.expectedError && err == nil {
				t.Errorf("expected error from createOrUpdateAffinityAssistantsPerWorkspace, but got no error")
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
					SecurityContext: &corev1.PodSecurityContext{
						RunAsNonRoot:   ptr.Bool(true),
						SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
					},
				},
			},
		},
	}

	stsWithOverridenTemplateFields := affinityAssistantStatefulSet(aa.AffinityAssistantPerWorkspace, "test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []string{}, containerConfigWithoutSecurityContext, nil)

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.NodeSelector) != 1 {
		t.Errorf("expected a NodeSelector in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("expected ImagePullSecrets in the StatefulSet")
	}

	if stsWithOverridenTemplateFields.Spec.Template.Spec.SecurityContext == nil {
		t.Errorf("expected a SecurityContext in the StatefulSet")
	}
}

func TestDefaultPodTemplatesArePropagatedToAffinityAssistant(t *testing.T) {
	prWithCustomPodTemplate := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "pipelinerun-with-custom-podtemplate",
		},
		Spec: v1.PipelineRunSpec{
			TaskRunTemplate: v1.PipelineTaskRunTemplate{
				PodTemplate: &pod.PodTemplate{
					HostNetwork: true,
				},
			},
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
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot:   ptr.Bool(true),
			SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault},
		},
	}

	stsWithOverridenTemplateFields := affinityAssistantStatefulSet(aa.AffinityAssistantPerWorkspace, "test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []string{}, containerConfigWithoutSecurityContext, defaultTpl)

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.NodeSelector) != 1 {
		t.Errorf("expected a NodeSelector in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.ImagePullSecrets) != 1 {
		t.Errorf("expected ImagePullSecrets in the StatefulSet")
	}

	if stsWithOverridenTemplateFields.Spec.Template.Spec.SecurityContext == nil {
		t.Errorf("expected SecurityContext in the StatefulSet")
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
					SecurityContext: &corev1.PodSecurityContext{RunAsNonRoot: ptr.Bool(true)},
				},
			},
		},
	}

	defaultTpl := &pod.AffinityAssistantTemplate{
		NodeSelector: map[string]string{
			"disktype": "ssd",
		},
		ImagePullSecrets: []corev1.LocalObjectReference{{
			Name: "reg-creds",
		}},
		SecurityContext: &corev1.PodSecurityContext{
			RunAsNonRoot: ptr.Bool(false),
		},
	}

	stsWithOverridenTemplateFields := affinityAssistantStatefulSet(aa.AffinityAssistantPerWorkspace, "test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []string{}, containerConfigWithoutSecurityContext, defaultTpl)

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations from spec in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.NodeSelector) != 1 {
		t.Errorf("expected NodeSelector from defaults in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.ImagePullSecrets) != 2 {
		t.Errorf("expected ImagePullSecrets from spec to overwrite default in the StatefulSet")
	}

	if stsWithOverridenTemplateFields.Spec.Template.Spec.SecurityContext.RunAsNonRoot == ptr.Bool(true) {
		t.Errorf("expected SecurityContext from spec to overwrite default in the StatefulSet")
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
				},
			},
		},
	}

	stsWithOverridenTemplateFields := affinityAssistantStatefulSet(aa.AffinityAssistantPerWorkspace, "test-assistant", prWithCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []string{}, containerConfigWithoutSecurityContext, nil)

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.Tolerations) != 1 {
		t.Errorf("expected Tolerations from spec in the StatefulSet")
	}

	if len(stsWithOverridenTemplateFields.Spec.Template.Spec.HostAliases) != 0 {
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

	stsWithoutTolerationsAndNodeSelector := affinityAssistantStatefulSet(aa.AffinityAssistantPerWorkspace, "test-assistant", prWithoutCustomPodTemplate, []corev1.PersistentVolumeClaim{}, []string{}, containerConfigWithoutSecurityContext, nil)

	if len(stsWithoutTolerationsAndNodeSelector.Spec.Template.Spec.Tolerations) != 0 {
		t.Errorf("unexpected Tolerations in the StatefulSet")
	}

	if len(stsWithoutTolerationsAndNodeSelector.Spec.Template.Spec.NodeSelector) != 0 {
		t.Errorf("unexpected NodeSelector in the StatefulSet")
	}

	if stsWithoutTolerationsAndNodeSelector.Spec.Template.Spec.SecurityContext != nil {
		t.Errorf("unexpected SecurityContext in the StatefulSet")
	}
}

// TestThatAffinityAssistantNameIsNoLongerThan53 tests that the Affinity Assistant Name
// is no longer than 53 chars. This is a limitation with StatefulSet.
// See https://github.com/kubernetes/kubernetes/issues/64023
// This is because the StatefulSet-controller adds a label with the name of the StatefulSet
// plus 10 chars for a hash. Labels in Kubernetes can not be longer than 63 chars.
// Typical output from the example below is affinity-assistant-0384086f62
func TestThatAffinityAssistantNameIsNoLongerThan53(t *testing.T) {
	affinityAssistantName := GetAffinityAssistantName(
		"pipeline-workspace-name-that-is-quite-long",
		"pipelinerun-with-a-long-custom-name")

	if len(affinityAssistantName) > 53 {
		t.Errorf("affinity assistant name can not be longer than 53 chars")
	}
}

func TestCleanupAffinityAssistants_Success(t *testing.T) {
	workspaces := []v1.WorkspaceBinding{
		{
			Name:                "volumeClaimTemplate workspace 1",
			VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
		},
		{
			Name:                "volumeClaimTemplate workspace 2",
			VolumeClaimTemplate: &corev1.PersistentVolumeClaim{},
		},
	}
	pr := &v1.PipelineRun{
		TypeMeta: metav1.TypeMeta{Kind: "PipelineRun"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pipelinerun-volumeclaimtemplate",
		},
		Spec: v1.PipelineRunSpec{
			Workspaces: workspaces,
		},
	}

	testCases := []struct {
		name                   string
		aaBehavior             aa.AffinityAssistantBehavior
		cfgMap                 map[string]string
		affinityAssistantNames []string
		pvcNames               []string
	}{{
		name:       "Affinity Assistant Cleanup - per workspace",
		aaBehavior: aa.AffinityAssistantPerWorkspace,
		cfgMap: map[string]string{
			"disable-affinity-assistant": "false",
			"coschedule":                 "workspaces",
		},
		affinityAssistantNames: []string{"affinity-assistant-9d8b15fa2e", "affinity-assistant-39883fc3b2"},
		pvcNames:               []string{"pvc-a12c589442", "pvc-5ce7cd98c5"},
	}, {
		name:       "Affinity Assistant Cleanup - per pipelinerun",
		aaBehavior: aa.AffinityAssistantPerPipelineRun,
		cfgMap: map[string]string{
			"disable-affinity-assistant": "true",
			"coschedule":                 "pipelineruns",
		},
		affinityAssistantNames: []string{"affinity-assistant-62843d388a"},
		pvcNames:               []string{"pvc-a12c589442-affinity-assistant-62843d388a-0", "pvc-5ce7cd98c5-affinity-assistant-62843d388a-0"},
	}}

	for _, tc := range testCases {
		// seed data to create StatefulSets and PVCs
		var statefulsets []*appsv1.StatefulSet
		for _, expectedAffinityAssistantName := range tc.affinityAssistantNames {
			ss := &appsv1.StatefulSet{
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
			}
			statefulsets = append(statefulsets, ss)
		}

		var pvcs []*corev1.PersistentVolumeClaim
		for _, expectedPVCName := range tc.pvcNames {
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name: expectedPVCName,
				},
			}
			pvcs = append(pvcs, pvc)
		}

		data := Data{
			StatefulSets: statefulsets,
			PVCs:         pvcs,
		}

		_, c, _ := seedTestData(data)
		ctx := cfgtesting.SetFeatureFlags(context.Background(), t, tc.cfgMap)

		// mocks `kubernetes.io/pvc-protection` finalizer behavior by adding DeletionTimestamp when deleting pvcs with the finalizer
		// see details in: https://kubernetes.io/docs/concepts/overview/working-with-objects/finalizers/#how-finalizers-work
		if tc.aaBehavior == aa.AffinityAssistantPerPipelineRun {
			for _, pvc := range pvcs {
				pvc.DeletionTimestamp = &metav1.Time{
					Time: metav1.Now().Time,
				}
				c.KubeClientSet.CoreV1().(*fake.FakeCoreV1).PrependReactor("delete", "persistentvolumeclaims",
					func(action testing2.Action) (handled bool, ret runtime.Object, err error) {
						return true, pvc, nil
					})
			}
		}

		// call clean up
		err := c.cleanupAffinityAssistantsAndPVCs(ctx, pr)
		if err != nil {
			t.Fatalf("unexpected err when clean up Affinity Assistant: %v", err)
		}

		// validate the cleanup result
		for _, expectedAffinityAssistantName := range tc.affinityAssistantNames {
			_, err = c.KubeClientSet.AppsV1().StatefulSets(pr.Namespace).Get(ctx, expectedAffinityAssistantName, metav1.GetOptions{})
			if !apierrors.IsNotFound(err) {
				t.Errorf("expected a NotFound response of StatefulSet, got: %v", err)
			}
		}

		// validate pvcs
		for _, expectedPVCName := range tc.pvcNames {
			if tc.aaBehavior == aa.AffinityAssistantPerWorkspace {
				//  the PVCs are NOT expected to be deleted at Affinity Assistant cleanup time in AffinityAssistantPerWorkspace mode
				_, err = c.KubeClientSet.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(ctx, expectedPVCName, metav1.GetOptions{})
				if err != nil {
					t.Errorf("unexpected err when getting PersistentVolumeClaims, err: %v", err)
				}
			} else {
				pvc, err := c.KubeClientSet.CoreV1().PersistentVolumeClaims(pr.Namespace).Get(ctx, expectedPVCName, metav1.GetOptions{})
				if err != nil {
					t.Fatalf("unexpect err when getting PersistentVolumeClaims, err: %v", err)
				}
				// validate the finalizer is removed, which mocks the pvc is deleted properly
				if len(pvc.Finalizers) > 0 {
					t.Errorf("pvc %s kubernetes.io/pvc-protection finalizer is not removed properly", pvc.Name)
				}
			}
		}
	}
}

func TestCleanupAffinityAssistantsAndPVCs_Failure(t *testing.T) {
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

	expectedErrs := errorutils.NewAggregate([]error{
		errors.New("failed to delete StatefulSet affinity-assistant-e3b0c44298: error deleting statefulsets"),
	})

	errs := c.cleanupAffinityAssistantsAndPVCs(ctx, pr)
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

	_ = c.cleanupAffinityAssistantsAndPVCs(store.ToContext(context.Background()), testPRWithPVC)

	if len(fakeClientSet.Actions()) != 0 {
		t.Errorf("Expected 0 k8s client requests, did %d request", len(fakeClientSet.Actions()))
	}
}

func TestGetAssistantAffinityMergedWithPodTemplateAffinity(t *testing.T) {
	labelSelector := &metav1.LabelSelector{
		MatchLabels: map[string]string{
			workspace.LabelComponent: workspace.ComponentNameAffinityAssistant,
		},
	}

	assistantWeightedPodAffinityTerm := corev1.WeightedPodAffinityTerm{
		Weight: 100,
		PodAffinityTerm: corev1.PodAffinityTerm{
			LabelSelector: labelSelector,
			TopologyKey:   "kubernetes.io/hostname",
		},
	}

	affinityWithAssistantAffinityPreferred := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				assistantWeightedPodAffinityTerm,
			},
		},
	}

	affinityWithAssistantAffinityRequired := &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{{
				LabelSelector: labelSelector,
				TopologyKey:   "kubernetes.io/hostname",
			}},
		},
	}

	prWithEmptyAffinityPodTemplate := parse.MustParseV1PipelineRun(t, `
metadata:
  name: pr-with-no-podTemplate
`)

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
				assistantWeightedPodAffinityTerm,
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
				assistantWeightedPodAffinityTerm,
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
		aaBehavior  aa.AffinityAssistantBehavior
		expect      *corev1.Affinity
	}{
		{
			description: "podTemplate affinity is empty - per workspace",
			pr:          prWithEmptyAffinityPodTemplate,
			aaBehavior:  aa.AffinityAssistantPerWorkspace,
			expect:      affinityWithAssistantAffinityPreferred,
		},
		{
			description: "podTemplate affinity is empty - per pipelineruns",
			pr:          prWithEmptyAffinityPodTemplate,
			aaBehavior:  aa.AffinityAssistantPerPipelineRun,
			expect:      affinityWithAssistantAffinityPreferred,
		},
		{
			description: "podTemplate affinity is empty - per isolate pipelinerun",
			pr:          prWithEmptyAffinityPodTemplate,
			aaBehavior:  aa.AffinityAssistantPerPipelineRunWithIsolation,
			expect:      affinityWithAssistantAffinityRequired,
		},
		{
			description: "podTemplate with affinity which contains podAntiAffinity",
			pr:          prWithPodTemplatePodAffinity,
			aaBehavior:  aa.AffinityAssistantPerWorkspace,
			expect:      affinityWithPodTemplatePodAffinity,
		},
		{
			description: "podTemplate with affinity which contains nodeAntiAffinity",
			pr:          prWithPodTemplateNodeAffinity,
			aaBehavior:  aa.AffinityAssistantPerWorkspace,
			expect:      affinityWithPodTemplateNodeAffinity,
		},
	} {
		t.Run(tc.description, func(t *testing.T) {
			resultAffinity := getAssistantAffinityMergedWithPodTemplateAffinity(tc.pr, tc.aaBehavior)
			if d := cmp.Diff(tc.expect, resultAffinity); d != "" {
				t.Errorf("affinity diff: %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestGetAffinityAssistantAnnotationVal(t *testing.T) {
	tcs := []struct {
		name                                                 string
		aaBehavior                                           aa.AffinityAssistantBehavior
		wsName, prName, expectAffinityAssistantAnnotationVal string
	}{{
		name:                                 "per workspace",
		aaBehavior:                           aa.AffinityAssistantPerWorkspace,
		wsName:                               "my-ws",
		prName:                               "my-pipelinerun",
		expectAffinityAssistantAnnotationVal: "affinity-assistant-315f58d30d",
	}, {
		name:       "per workspace - empty pipeline workspace name",
		aaBehavior: aa.AffinityAssistantPerWorkspace,
		prName:     "my-pipelinerun",
	}, {
		name:                                 "per pipelinerun",
		aaBehavior:                           aa.AffinityAssistantPerPipelineRun,
		wsName:                               "my-ws",
		prName:                               "my-pipelinerun",
		expectAffinityAssistantAnnotationVal: "affinity-assistant-0b79942a50",
	}, {
		name:                                 "isolate pipelinerun",
		aaBehavior:                           aa.AffinityAssistantPerPipelineRunWithIsolation,
		wsName:                               "my-ws",
		prName:                               "my-pipelinerun",
		expectAffinityAssistantAnnotationVal: "affinity-assistant-0b79942a50",
	}, {
		name:       "disabled",
		aaBehavior: aa.AffinityAssistantDisabled,
		wsName:     "my-ws",
		prName:     "my-pipelinerun",
	}}

	for _, tc := range tcs {
		aaAnnotationVal := getAffinityAssistantAnnotationVal(tc.aaBehavior, tc.wsName, tc.prName)
		if diff := cmp.Diff(tc.expectAffinityAssistantAnnotationVal, aaAnnotationVal); diff != "" {
			t.Errorf("Affinity Assistant Annotation Val mismatch: %v", diff)
		}
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

	kubeClientSet := fakek8s.NewSimpleClientset()
	c := Reconciler{
		KubeClientSet: kubeClientSet,
		pvcHandler:    volumeclaim.NewPVCHandler(kubeClientSet, zap.NewExample().Sugar()),
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

func validateStatefulSetSpec(t *testing.T, ctx context.Context, c Reconciler, expectAAName string, expectStatefulSetSpec *appsv1.StatefulSetSpec) {
	t.Helper()
	aa, err := c.KubeClientSet.AppsV1().StatefulSets("").Get(ctx, expectAAName, metav1.GetOptions{})
	if expectStatefulSetSpec != nil {
		if err != nil {
			t.Fatalf("unexpected error when retrieving StatefulSet: %v", err)
		}
		if d := cmp.Diff(expectStatefulSetSpec, &aa.Spec, podSpecFilter, podTemplateSpecFilter, podContainerFilter); d != "" {
			t.Errorf("StatefulSetSpec diff: %s", diff.PrintWantGot(d))
		}
	} else if !apierrors.IsNotFound(err) {
		t.Errorf("unexpected error when retrieving StatefulSet which expects nil: %v", err)
	}
}
