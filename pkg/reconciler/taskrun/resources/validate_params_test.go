package resources_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
)

func TestValidateParamArrayIndex(t *testing.T) {
	stepsInvalidReferences := []string{}
	for i := 10; i <= 26; i++ {
		stepsInvalidReferences = append(stepsInvalidReferences, fmt.Sprintf("$(params.array-params[%d])", i))
	}
	volumesInvalidReferences := []string{}
	for i := 10; i <= 22; i++ {
		volumesInvalidReferences = append(volumesInvalidReferences, fmt.Sprintf("$(params.array-params[%d])", i))
	}

	tcs := []struct {
		name          string
		params        v1.Params
		taskspec      *v1.TaskSpec
		expectedError error
	}{{
		name: "steps reference invalid",
		params: v1.Params{{
			Name:  "array-params",
			Value: *v1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1.TaskSpec{
			Params: []v1.ParamSpec{{
				Name:    "array-params",
				Default: v1.NewStructuredValues("bar", "foo"),
			}},
			Steps: []v1.Step{{
				Name:    "$(params.array-params[10])",
				Image:   "$(params.array-params[11])",
				Command: []string{"$(params.array-params[12])"},
				Args:    []string{"$(params.array-params[13])"},
				Script:  "echo $(params.array-params[14])",
				Env: []corev1.EnvVar{{
					Value: "$(params.array-params[15])",
					ValueFrom: &corev1.EnvVarSource{
						SecretKeyRef: &corev1.SecretKeySelector{
							Key: "$(params.array-params[16])",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.array-params[17])",
							},
						},
						ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
							Key: "$(params.array-params[18])",
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.array-params[19])",
							},
						},
					},
				}},
				EnvFrom: []corev1.EnvFromSource{{
					Prefix: "$(params.array-params[20])",
					ConfigMapRef: &corev1.ConfigMapEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(params.array-params[21])",
						},
					},
					SecretRef: &corev1.SecretEnvSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(params.array-params[22])",
						},
					},
				}},
				WorkingDir: "$(params.array-params[23])",
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "$(params.array-params[24])",
					MountPath: "$(params.array-params[25])",
					SubPath:   "$(params.array-params[26])",
				}},
			}},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", strings.Join(stepsInvalidReferences, " ")),
	}, {
		name: "stepTemplate reference invalid",
		params: v1.Params{{
			Name:  "array-params",
			Value: *v1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1.TaskSpec{
			Params: []v1.ParamSpec{{
				Name:    "array-params",
				Default: v1.NewStructuredValues("bar", "foo"),
			}},
			StepTemplate: &v1.StepTemplate{
				Image: "$(params.array-params[3])",
			},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", "$(params.array-params[3])"),
	}, {
		name: "volumes reference invalid",
		params: v1.Params{{
			Name:  "array-params",
			Value: *v1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1.TaskSpec{
			Params: []v1.ParamSpec{{
				Name:    "array-params",
				Default: v1.NewStructuredValues("bar", "foo"),
			}},
			Volumes: []corev1.Volume{{
				Name: "$(params.array-params[10])",
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: "$(params.array-params[11])",
						},
						Items: []corev1.KeyToPath{{
							Key:  "$(params.array-params[12])",
							Path: "$(params.array-params[13])",
						},
						},
					},
					Secret: &corev1.SecretVolumeSource{
						SecretName: "$(params.array-params[14])",
						Items: []corev1.KeyToPath{{
							Key:  "$(params.array-params[15])",
							Path: "$(params.array-params[16])",
						}},
					},
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "$(params.array-params[17])",
					},
					Projected: &corev1.ProjectedVolumeSource{
						Sources: []corev1.VolumeProjection{{
							ConfigMap: &corev1.ConfigMapProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "$(params.array-params[18])",
								},
							},
							Secret: &corev1.SecretProjection{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: "$(params.array-params[19])",
								},
							},
							ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
								Audience: "$(params.array-params[20])",
							},
						}},
					},
					CSI: &corev1.CSIVolumeSource{
						NodePublishSecretRef: &corev1.LocalObjectReference{
							Name: "$(params.array-params[21])",
						},
						VolumeAttributes: map[string]string{"key": "$(params.array-params[22])"},
					},
				},
			},
			},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", strings.Join(volumesInvalidReferences, " ")),
	}, {
		name: "workspaces reference invalid",
		params: v1.Params{{
			Name:  "array-params",
			Value: *v1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1.TaskSpec{
			Params: []v1.ParamSpec{{
				Name:    "array-params",
				Default: v1.NewStructuredValues("bar", "foo"),
			}},
			Workspaces: []v1.WorkspaceDeclaration{{
				MountPath: "$(params.array-params[3])",
			}},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", "$(params.array-params[3])"),
	}, {
		name: "sidecar reference invalid",
		params: v1.Params{{
			Name:  "array-params",
			Value: *v1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1.TaskSpec{
			Params: []v1.ParamSpec{{
				Name:    "array-params",
				Default: v1.NewStructuredValues("bar", "foo"),
			}},
			Sidecars: []v1.Sidecar{{
				Script: "$(params.array-params[3])",
			},
			},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", "$(params.array-params[3])"),
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := resources.ValidateParamArrayIndex(tc.taskspec, tc.params)
			if d := cmp.Diff(tc.expectedError.Error(), err.Error()); d != "" {
				t.Errorf("validateParamArrayIndex() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
