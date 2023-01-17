/*
Copyright 2019 The Tekton Authors

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

package taskrun

import (
	"context"
	"fmt"
	"strings"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestValidateOverrides(t *testing.T) {
	tcs := []struct {
		name    string
		ts      *v1beta1.TaskSpec
		trs     *v1beta1.TaskRunSpec
		wantErr bool
	}{{
		name: "valid stepOverrides",
		ts: &v1beta1.TaskSpec{
			Steps: []v1beta1.Step{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1beta1.TaskRunSpec{
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "step1",
			}},
		},
	}, {
		name: "valid sidecarOverrides",
		ts: &v1beta1.TaskSpec{
			Sidecars: []v1beta1.Sidecar{{
				Name: "step1",
			}, {
				Name: "step2",
			}},
		},
		trs: &v1beta1.TaskRunSpec{
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "step1",
			}},
		},
	}, {
		name: "invalid stepOverrides",
		ts:   &v1beta1.TaskSpec{},
		trs: &v1beta1.TaskRunSpec{
			StepOverrides: []v1beta1.TaskRunStepOverride{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}, {
		name: "invalid sidecarOverrides",
		ts:   &v1beta1.TaskSpec{},
		trs: &v1beta1.TaskRunSpec{
			SidecarOverrides: []v1beta1.TaskRunSidecarOverride{{
				Name: "step1",
			}},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateOverrides(tc.ts, tc.trs)
			if (err != nil) != tc.wantErr {
				t.Errorf("expected err: %t, but got err %s", tc.wantErr, err)
			}
		})
	}
}

func TestValidateResult(t *testing.T) {
	tcs := []struct {
		name    string
		tr      *v1beta1.TaskRun
		rtr     *v1beta1.TaskSpec
		wantErr bool
	}{{
		name: "valid taskrun spec results",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "valid taskspec results",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name: "object-result",
							Type: v1beta1.ResultsTypeObject,
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: false,
	}, {
		name: "invalid taskrun spec results types",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name: "string-result",
							Type: v1beta1.ResultsTypeString,
						},
						{
							Name: "array-result",
							Type: v1beta1.ResultsTypeArray,
						},
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results types",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "string-result",
							Type:  v1beta1.ResultsTypeArray,
							Value: *v1beta1.NewStructuredValues("hello", "world"),
						},
						{
							Name:  "array-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeString,
							Value: *v1beta1.NewStructuredValues("hello"),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{
				{
					Name: "string-result",
					Type: v1beta1.ResultsTypeString,
				},
				{
					Name: "array-result",
					Type: v1beta1.ResultsTypeArray,
				},
				{
					Name:       "object-result",
					Type:       v1beta1.ResultsTypeObject,
					Properties: map[string]v1beta1.PropertySpec{"hello": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}, {
		name: "invalid taskrun spec results object properties",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{
						{
							Name:       "object-result",
							Type:       v1beta1.ResultsTypeObject,
							Properties: map[string]v1beta1.PropertySpec{"world": {Type: "string"}},
						},
					},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{},
		},
		wantErr: true,
	}, {
		name: "invalid taskspec results object properties",
		tr: &v1beta1.TaskRun{
			Spec: v1beta1.TaskRunSpec{
				TaskSpec: &v1beta1.TaskSpec{
					Results: []v1beta1.TaskResult{},
				},
			},
			Status: v1beta1.TaskRunStatus{
				TaskRunStatusFields: v1beta1.TaskRunStatusFields{
					TaskRunResults: []v1beta1.TaskRunResult{
						{
							Name:  "object-result",
							Type:  v1beta1.ResultsTypeObject,
							Value: *v1beta1.NewObject(map[string]string{"hello": "world"}),
						},
					},
				},
			},
		},
		rtr: &v1beta1.TaskSpec{
			Results: []v1beta1.TaskResult{
				{
					Name:       "object-result",
					Type:       v1beta1.ResultsTypeObject,
					Properties: map[string]v1beta1.PropertySpec{"world": {Type: "string"}},
				},
			},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := validateTaskRunResults(tc.tr, tc.rtr)
			if (err != nil) != tc.wantErr {
				t.Errorf("expected err: %t, but got err %s", tc.wantErr, err)
			}
		})
	}
}

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
		params        []v1beta1.Param
		taskspec      *v1beta1.TaskSpec
		expectedError error
	}{{
		name: "steps reference invalid",
		params: []v1beta1.Param{{
			Name:  "array-params",
			Value: *v1beta1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Steps: []v1beta1.Step{{
				Name:    "$(params.array-params[10])",
				Image:   "$(params.array-params[11])",
				Command: []string{"$(params.array-params[12])"},
				Args:    []string{"$(params.array-params[13])"},
				Script:  "echo $(params.array-params[14])",
				Env: []v1.EnvVar{{
					Value: "$(params.array-params[15])",
					ValueFrom: &v1.EnvVarSource{
						SecretKeyRef: &v1.SecretKeySelector{
							Key: "$(params.array-params[16])",
							LocalObjectReference: v1.LocalObjectReference{
								Name: "$(params.array-params[17])",
							},
						},
						ConfigMapKeyRef: &v1.ConfigMapKeySelector{
							Key: "$(params.array-params[18])",
							LocalObjectReference: v1.LocalObjectReference{
								Name: "$(params.array-params[19])",
							},
						},
					},
				}},
				EnvFrom: []v1.EnvFromSource{{
					Prefix: "$(params.array-params[20])",
					ConfigMapRef: &v1.ConfigMapEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "$(params.array-params[21])",
						},
					},
					SecretRef: &v1.SecretEnvSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "$(params.array-params[22])",
						},
					},
				}},
				WorkingDir: "$(params.array-params[23])",
				VolumeMounts: []v1.VolumeMount{{
					Name:      "$(params.array-params[24])",
					MountPath: "$(params.array-params[25])",
					SubPath:   "$(params.array-params[26])",
				}},
			}},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", strings.Join(stepsInvalidReferences, " ")),
	}, {
		name: "stepTemplate reference invalid",
		params: []v1beta1.Param{{
			Name:  "array-params",
			Value: *v1beta1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			StepTemplate: &v1beta1.StepTemplate{
				Image: "$(params.array-params[3])",
			},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", "$(params.array-params[3])"),
	}, {
		name: "volumes reference invalid",
		params: []v1beta1.Param{{
			Name:  "array-params",
			Value: *v1beta1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Volumes: []v1.Volume{{
				Name: "$(params.array-params[10])",
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: "$(params.array-params[11])",
						},
						Items: []v1.KeyToPath{{
							Key:  "$(params.array-params[12])",
							Path: "$(params.array-params[13])",
						},
						},
					},
					Secret: &v1.SecretVolumeSource{
						SecretName: "$(params.array-params[14])",
						Items: []v1.KeyToPath{{
							Key:  "$(params.array-params[15])",
							Path: "$(params.array-params[16])",
						}},
					},
					PersistentVolumeClaim: &v1.PersistentVolumeClaimVolumeSource{
						ClaimName: "$(params.array-params[17])",
					},
					Projected: &v1.ProjectedVolumeSource{
						Sources: []v1.VolumeProjection{{
							ConfigMap: &v1.ConfigMapProjection{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "$(params.array-params[18])",
								},
							},
							Secret: &v1.SecretProjection{
								LocalObjectReference: v1.LocalObjectReference{
									Name: "$(params.array-params[19])",
								},
							},
							ServiceAccountToken: &v1.ServiceAccountTokenProjection{
								Audience: "$(params.array-params[20])",
							},
						}},
					},
					CSI: &v1.CSIVolumeSource{
						NodePublishSecretRef: &v1.LocalObjectReference{
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
		params: []v1beta1.Param{{
			Name:  "array-params",
			Value: *v1beta1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Workspaces: []v1beta1.WorkspaceDeclaration{{
				MountPath: "$(params.array-params[3])",
			}},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", "$(params.array-params[3])"),
	}, {
		name: "sidecar reference invalid",
		params: []v1beta1.Param{{
			Name:  "array-params",
			Value: *v1beta1.NewStructuredValues("bar", "foo"),
		}},
		taskspec: &v1beta1.TaskSpec{
			Params: []v1beta1.ParamSpec{{
				Name:    "array-params",
				Default: v1beta1.NewStructuredValues("bar", "foo"),
			}},
			Sidecars: []v1beta1.Sidecar{{
				Script: "$(params.array-params[3])",
			},
			},
		},
		expectedError: fmt.Errorf("non-existent param references:[%v]", "$(params.array-params[3])"),
	},
	}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{FeatureFlags: &config.FeatureFlags{EnableAPIFields: "alpha"}})
			err := validateParamArrayIndex(ctx, tc.params, tc.taskspec)
			if d := cmp.Diff(tc.expectedError.Error(), err.Error()); d != "" {
				t.Errorf("validateParamArrayIndex() errors diff %s", diff.PrintWantGot(d))
			}
		})
	}
}
