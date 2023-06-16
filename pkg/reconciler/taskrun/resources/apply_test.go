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

package resources_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun/resources"
	"github.com/tektoncd/pipeline/pkg/workspace"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/names"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	simpleTaskSpec = &v1.TaskSpec{
		Sidecars: []v1.Sidecar{{
			Name:  "foo",
			Image: `$(params["myimage"])`,
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "$(params['FOO'])",
			}},
		}},
		StepTemplate: &v1.StepTemplate{
			Env: []corev1.EnvVar{{
				Name:  "template-var",
				Value: `$(params["FOO"])`,
			}},
			Image: "$(params.myimage)",
		},
		Steps: []v1.Step{{
			Name:  "foo",
			Image: "$(params.myimage)",
		}, {
			Name:       "baz",
			Image:      "bat",
			WorkingDir: "$(inputs.resources.workspace.path)",
			Args:       []string{"$(inputs.resources.workspace.url)"},
		}, {
			Name:  "foo",
			Image: `$(params["myimage"])`,
		}, {
			Name:       "baz",
			Image:      "$(params.somethingelse)",
			WorkingDir: "$(inputs.resources.workspace.path)",
			Args:       []string{"$(inputs.resources.workspace.url)"},
		}, {
			Name:  "foo",
			Image: "busybox:$(params.FOO)",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.FOO)",
				MountPath: "path/to/$(params.FOO)",
				SubPath:   "sub/$(params.FOO)/path",
			}},
		}, {
			Name:  "foo",
			Image: "busybox:$(params.FOO)",
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "value-$(params.FOO)",
			}, {
				Name: "bar",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(params.FOO)"},
						Key:                  "config-key-$(params.FOO)",
					},
				},
			}, {
				Name: "baz",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(params.FOO)"},
						Key:                  "secret-key-$(params.FOO)",
					},
				},
			}},
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "prefix-0-$(params.FOO)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(params.FOO)"},
				},
			}, {
				Prefix: "prefix-1-$(params.FOO)",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(params.FOO)"},
				},
			}},
		}},
		Volumes: []corev1.Volume{{
			Name: "$(params.FOO)",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(params.FOO)",
					},
					Items: []corev1.KeyToPath{{
						Key:  "$(params.FOO)",
						Path: "$(params.FOO)",
					}},
				},
			},
		}, {
			Name: "some-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "$(params.FOO)",
					Items: []corev1.KeyToPath{{
						Key:  "$(params.FOO)",
						Path: "$(params.FOO)",
					}},
				},
			},
		}, {
			Name: "some-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "$(params.FOO)",
				},
			},
		}, {
			Name: "some-projected-volumes",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.FOO)",
							},
						},
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.FOO)",
							},
						},
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience: "$(params.FOO)",
						},
					}},
				},
			},
		}, {
			Name: "some-csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					VolumeAttributes: map[string]string{
						"secretProviderClass": "$(params.FOO)",
					},
					NodePublishSecretRef: &corev1.LocalObjectReference{
						Name: "$(params.FOO)",
					},
				},
			},
		}},
	}

	// a taskspec for testing object var in all places i.e. Sidecars, StepTemplate, Steps and Volumns
	objectParamTaskSpec = &v1.TaskSpec{
		Sidecars: []v1.Sidecar{{
			Name:  "foo",
			Image: `$(params.myObject.key1)`,
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "$(params.myObject.key2)",
			}},
		}},
		StepTemplate: &v1.StepTemplate{
			Image: "$(params.myObject.key1)",
			Env: []corev1.EnvVar{{
				Name:  "template-var",
				Value: `$(params.myObject.key2)`,
			}},
		},
		Steps: []v1.Step{{
			Name:       "foo",
			Image:      "$(params.myObject.key1)",
			WorkingDir: "path/to/$(params.myObject.key2)",
			Args:       []string{"first $(params.myObject.key1)", "second $(params.myObject.key2)"},
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.myObject.key1)",
				MountPath: "path/to/$(params.myObject.key2)",
				SubPath:   "sub/$(params.myObject.key2)/path",
			}},
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "value-$(params.myObject.key1)",
			}, {
				Name: "bar",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(params.myObject.key1)"},
						Key:                  "config-key-$(params.myObject.key2)",
					},
				},
			}, {
				Name: "baz",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(params.myObject.key1)"},
						Key:                  "secret-key-$(params.myObject.key2)",
					},
				},
			}},
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "prefix-0-$(params.myObject.key1)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(params.myObject.key1)"},
				},
			}, {
				Prefix: "prefix-1-$(params.myObject.key1)",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(params.myObject.key1)"},
				},
			}},
		}},
		Volumes: []corev1.Volume{{
			Name: "$(params.myObject.key1)",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(params.myObject.key1)",
					},
					Items: []corev1.KeyToPath{{
						Key:  "$(params.myObject.key1)",
						Path: "$(params.myObject.key2)",
					}},
				},
				Secret: &corev1.SecretVolumeSource{
					SecretName: "$(params.myObject.key1)",
					Items: []corev1.KeyToPath{{
						Key:  "$(params.myObject.key1)",
						Path: "$(params.myObject.key2)",
					}},
				},
			},
		}, {
			Name: "some-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "$(params.myObject.key1)",
				},
			},
		}, {
			Name: "some-projected-volumes",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.myObject.key1)",
							},
						},
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.myObject.key1)",
							},
						},
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience: "$(params.myObject.key2)",
						},
					}},
				},
			},
		}, {
			Name: "some-csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					VolumeAttributes: map[string]string{
						"secretProviderClass": "$(params.myObject.key1)",
					},
					NodePublishSecretRef: &corev1.LocalObjectReference{
						Name: "$(params.myObject.key1)",
					},
				},
			},
		}},
	}

	simpleTaskSpecArrayIndexing = &v1.TaskSpec{
		Sidecars: []v1.Sidecar{{
			Name:  "foo",
			Image: `$(params["myimage"][0])`,
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "$(params['FOO'][1])",
			}},
		}},
		StepTemplate: &v1.StepTemplate{
			Env: []corev1.EnvVar{{
				Name:  "template-var",
				Value: `$(params["FOO"][1])`,
			}},
			Image: "$(params.myimage[0])",
		},
		Steps: []v1.Step{{
			Name:  "foo",
			Image: "$(params.myimage[0])",
		}, {
			Name:       "baz",
			Image:      "bat",
			WorkingDir: "$(inputs.resources.workspace.path)",
			Args:       []string{"$(inputs.resources.workspace.url)"},
		}, {
			Name:  "foo",
			Image: `$(params["myimage"][0])`,
		}, {
			Name:       "baz",
			Image:      "$(params.somethingelse)",
			WorkingDir: "$(inputs.resources.workspace.path)",
			Args:       []string{"$(inputs.resources.workspace.url)"},
		}, {
			Name:  "foo",
			Image: "busybox:$(params.FOO[1])",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(params.FOO[1])",
				MountPath: "path/to/$(params.FOO[1])",
				SubPath:   "sub/$(params.FOO[1])/path",
			}},
		}, {
			Name:  "foo",
			Image: "busybox:$(params.FOO[1])",
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "value-$(params.FOO[1])",
			}, {
				Name: "bar",
				ValueFrom: &corev1.EnvVarSource{
					ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(params.FOO[1])"},
						Key:                  "config-key-$(params.FOO[1])",
					},
				},
			}, {
				Name: "baz",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(params.FOO[1])"},
						Key:                  "secret-key-$(params.FOO[1])",
					},
				},
			}},
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "prefix-0-$(params.FOO[1])",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "config-$(params.FOO[1])"},
				},
			}, {
				Prefix: "prefix-1-$(params.FOO[1])",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "secret-$(params.FOO[1])"},
				},
			}},
		}},
		Volumes: []corev1.Volume{{
			Name: "$(params.FOO[1])",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(params.FOO[1])",
					},
					Items: []corev1.KeyToPath{{
						Key:  "$(params.FOO[1])",
						Path: "$(params.FOO[1])",
					}},
				},
			},
		}, {
			Name: "some-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "$(params.FOO[1])",
					Items: []corev1.KeyToPath{{
						Key:  "$(params.FOO[1])",
						Path: "$(params.FOO[1])",
					}},
				},
			},
		}, {
			Name: "some-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "$(params.FOO[1])",
				},
			},
		}, {
			Name: "some-projected-volumes",
			VolumeSource: corev1.VolumeSource{
				Projected: &corev1.ProjectedVolumeSource{
					Sources: []corev1.VolumeProjection{{
						ConfigMap: &corev1.ConfigMapProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.FOO[1])",
							},
						},
						Secret: &corev1.SecretProjection{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "$(params.FOO[1])",
							},
						},
						ServiceAccountToken: &corev1.ServiceAccountTokenProjection{
							Audience: "$(params.FOO[1])",
						},
					}},
				},
			},
		}, {
			Name: "some-csi",
			VolumeSource: corev1.VolumeSource{
				CSI: &corev1.CSIVolumeSource{
					VolumeAttributes: map[string]string{
						"secretProviderClass": "$(params.FOO[1])",
					},
					NodePublishSecretRef: &corev1.LocalObjectReference{
						Name: "$(params.FOO[1])",
					},
				},
			},
		}},
	}

	arrayParamTaskSpec = &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "simple-image",
			Image: "some-image",
		}, {
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"echo"},
			Args:    []string{"first", "second", "$(params.array-param)", "last"},
		}},
	}

	arrayAndStringParamTaskSpec = &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "simple-image",
			Image: "some-image",
		}, {
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"echo"},
			Args:    []string{"$(params.normal-param)", "second", "$(params.array-param)", "last"},
		}},
	}

	arrayAndObjectParamTaskSpec = &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "simple-image",
			Image: "some-image",
		}, {
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"echo"},
			Args:    []string{"$(params.myObject.key1)", "$(params.myObject.key2)", "$(params.array-param)", "last"},
		}},
	}

	multipleArrayParamsTaskSpec = &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "simple-image",
			Image: "some-image",
		}, {
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"cmd", "$(params.another-array-param)"},
			Args:    []string{"first", "second", "$(params.array-param)", "last"},
		}},
	}

	multipleArrayAndStringsParamsTaskSpec = &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "simple-image",
			Image: "image-$(params.string-param2)",
		}, {
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"cmd", "$(params.array-param1)"},
			Args:    []string{"$(params.array-param2)", "second", "$(params.array-param1)", "$(params.string-param1)", "last"},
		}},
	}

	multipleArrayAndObjectParamsTaskSpec = &v1.TaskSpec{
		Steps: []v1.Step{{
			Name:  "simple-image",
			Image: "image-$(params.myObject.key1)",
		}, {
			Name:    "image-with-c-specified",
			Image:   "some-other-image",
			Command: []string{"cmd", "$(params.array-param1)"},
			Args:    []string{"$(params.array-param2)", "second", "$(params.array-param1)", "$(params.myObject.key2)", "last"},
		}},
	}

	arrayTaskRun0Elements = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name: "array-param",
				Value: v1.ParamValue{
					Type:     v1.ParamTypeArray,
					ArrayVal: []string{},
				}},
			},
		},
	}

	arrayTaskRun1Elements = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param",
				Value: *v1.NewStructuredValues("foo"),
			}},
		},
	}

	arrayTaskRun3Elements = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param",
				Value: *v1.NewStructuredValues("foo", "bar", "third"),
			}},
		},
	}

	arrayTaskRunMultipleArrays = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param",
				Value: *v1.NewStructuredValues("foo", "bar", "third"),
			}, {
				Name:  "another-array-param",
				Value: *v1.NewStructuredValues("part1", "part2"),
			}},
		},
	}

	arrayTaskRunWith1StringParam = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param",
				Value: *v1.NewStructuredValues("middlefirst", "middlesecond"),
			}, {
				Name:  "normal-param",
				Value: *v1.NewStructuredValues("foo"),
			}},
		},
	}

	arrayTaskRunWith1ObjectParam = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param",
				Value: *v1.NewStructuredValues("middlefirst", "middlesecond"),
			}, {
				Name: "myObject",
				Value: *v1.NewObject(map[string]string{
					"key1": "object value1",
					"key2": "object value2",
				}),
			}},
		},
	}

	arrayTaskRunMultipleArraysAndStrings = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param1",
				Value: *v1.NewStructuredValues("1-param1", "2-param1", "3-param1", "4-param1"),
			}, {
				Name:  "array-param2",
				Value: *v1.NewStructuredValues("1-param2", "2-param2", "2-param3"),
			}, {
				Name:  "string-param1",
				Value: *v1.NewStructuredValues("foo"),
			}, {
				Name:  "string-param2",
				Value: *v1.NewStructuredValues("bar"),
			}},
		},
	}

	arrayTaskRunMultipleArraysAndObject = &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "array-param1",
				Value: *v1.NewStructuredValues("1-param1", "2-param1", "3-param1", "4-param1"),
			}, {
				Name:  "array-param2",
				Value: *v1.NewStructuredValues("1-param2", "2-param2", "3-param3"),
			}, {
				Name: "myObject",
				Value: *v1.NewObject(map[string]string{
					"key1": "value1",
					"key2": "value2",
				}),
			}},
		},
	}
)

func applyMutation(ts *v1.TaskSpec, f func(*v1.TaskSpec)) *v1.TaskSpec {
	ts = ts.DeepCopy()
	f(ts)
	return ts
}

func TestApplyArrayParameters(t *testing.T) {
	type args struct {
		ts *v1.TaskSpec
		tr *v1.TaskRun
		dp []v1.ParamSpec
	}
	tests := []struct {
		name string
		args args
		want *v1.TaskSpec
	}{{
		name: "array parameter with 0 elements",
		args: args{
			ts: arrayParamTaskSpec,
			tr: arrayTaskRun0Elements,
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "last"}
		}),
	}, {
		name: "array parameter with 1 element",
		args: args{
			ts: arrayParamTaskSpec,
			tr: arrayTaskRun1Elements,
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "foo", "last"}
		}),
	}, {
		name: "array parameter with 3 elements",
		args: args{
			ts: arrayParamTaskSpec,
			tr: arrayTaskRun3Elements,
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "foo", "bar", "third", "last"}
		}),
	}, {
		name: "multiple arrays",
		args: args{
			ts: multipleArrayParamsTaskSpec,
			tr: arrayTaskRunMultipleArrays,
		},
		want: applyMutation(multipleArrayParamsTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Command = []string{"cmd", "part1", "part2"}
			spec.Steps[1].Args = []string{"first", "second", "foo", "bar", "third", "last"}
		}),
	}, {
		name: "array and normal string parameter",
		args: args{
			ts: arrayAndStringParamTaskSpec,
			tr: arrayTaskRunWith1StringParam,
		},
		want: applyMutation(arrayAndStringParamTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Args = []string{"foo", "second", "middlefirst", "middlesecond", "last"}
		}),
	}, {
		name: "several arrays and strings",
		args: args{
			ts: multipleArrayAndStringsParamsTaskSpec,
			tr: arrayTaskRunMultipleArraysAndStrings,
		},
		want: applyMutation(multipleArrayAndStringsParamsTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[0].Image = "image-bar"
			spec.Steps[1].Command = []string{"cmd", "1-param1", "2-param1", "3-param1", "4-param1"}
			spec.Steps[1].Args = []string{"1-param2", "2-param2", "2-param3", "second", "1-param1", "2-param1", "3-param1", "4-param1", "foo", "last"}
		}),
	}, {
		name: "array and object parameter",
		args: args{
			ts: arrayAndObjectParamTaskSpec,
			tr: arrayTaskRunWith1ObjectParam,
		},
		want: applyMutation(arrayAndObjectParamTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Args = []string{"object value1", "object value2", "middlefirst", "middlesecond", "last"}
		}),
	}, {
		name: "several arrays and objects",
		args: args{
			ts: multipleArrayAndObjectParamsTaskSpec,
			tr: arrayTaskRunMultipleArraysAndObject,
		},
		want: applyMutation(multipleArrayAndObjectParamsTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[0].Image = "image-value1"
			spec.Steps[1].Command = []string{"cmd", "1-param1", "2-param1", "3-param1", "4-param1"}
			spec.Steps[1].Args = []string{"1-param2", "2-param2", "3-param3", "second", "1-param1", "2-param1", "3-param1", "4-param1", "value2", "last"}
		}),
	}, {
		name: "default array parameter",
		args: args{
			ts: arrayParamTaskSpec,
			tr: &v1.TaskRun{},
			dp: []v1.ParamSpec{{
				Name:    "array-param",
				Default: v1.NewStructuredValues("defaulted", "value!"),
			}},
		},
		want: applyMutation(arrayParamTaskSpec, func(spec *v1.TaskSpec) {
			spec.Steps[1].Args = []string{"first", "second", "defaulted", "value!", "last"}
		}),
	}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resources.ApplyParameters(context.Background(), tt.args.ts, tt.args.tr, tt.args.dp...)
			if d := cmp.Diff(tt.want, got); d != "" {
				t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyParameters(t *testing.T) {
	tr := &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "myimage",
				Value: *v1.NewStructuredValues("bar"),
			}, {
				Name:  "FOO",
				Value: *v1.NewStructuredValues("world"),
			}},
		},
	}
	dp := []v1.ParamSpec{{
		Name:    "something",
		Default: v1.NewStructuredValues("mydefault"),
	}, {
		Name:    "somethingelse",
		Default: v1.NewStructuredValues(""),
	}}
	want := applyMutation(simpleTaskSpec, func(spec *v1.TaskSpec) {
		spec.StepTemplate.Env[0].Value = "world"
		spec.StepTemplate.Image = "bar"

		spec.Steps[0].Image = "bar"
		spec.Steps[2].Image = "mydefault"
		spec.Steps[2].Image = "bar"
		spec.Steps[3].Image = ""

		spec.Steps[4].VolumeMounts[0].Name = "world"
		spec.Steps[4].VolumeMounts[0].SubPath = "sub/world/path"
		spec.Steps[4].VolumeMounts[0].MountPath = "path/to/world"
		spec.Steps[4].Image = "busybox:world"

		spec.Steps[5].Env[0].Value = "value-world"
		spec.Steps[5].Env[1].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = "config-world"
		spec.Steps[5].Env[1].ValueFrom.ConfigMapKeyRef.Key = "config-key-world"
		spec.Steps[5].Env[2].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "secret-world"
		spec.Steps[5].Env[2].ValueFrom.SecretKeyRef.Key = "secret-key-world"
		spec.Steps[5].EnvFrom[0].Prefix = "prefix-0-world"
		spec.Steps[5].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "config-world"
		spec.Steps[5].EnvFrom[1].Prefix = "prefix-1-world"
		spec.Steps[5].EnvFrom[1].SecretRef.LocalObjectReference.Name = "secret-world"
		spec.Steps[5].Image = "busybox:world"

		spec.Volumes[0].Name = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Key = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Path = "world"
		spec.Volumes[1].VolumeSource.Secret.SecretName = "world"
		spec.Volumes[1].VolumeSource.Secret.Items[0].Key = "world"
		spec.Volumes[1].VolumeSource.Secret.Items[0].Path = "world"
		spec.Volumes[2].VolumeSource.PersistentVolumeClaim.ClaimName = "world"
		spec.Volumes[3].VolumeSource.Projected.Sources[0].ConfigMap.Name = "world"
		spec.Volumes[3].VolumeSource.Projected.Sources[0].Secret.Name = "world"
		spec.Volumes[3].VolumeSource.Projected.Sources[0].ServiceAccountToken.Audience = "world"
		spec.Volumes[4].VolumeSource.CSI.VolumeAttributes["secretProviderClass"] = "world"
		spec.Volumes[4].VolumeSource.CSI.NodePublishSecretRef.Name = "world"

		spec.Sidecars[0].Image = "bar"
		spec.Sidecars[0].Env[0].Value = "world"
	})
	got := resources.ApplyParameters(context.Background(), simpleTaskSpec, tr, dp...)
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
	}
}

func TestApplyParameters_ArrayIndexing(t *testing.T) {
	tr := &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name:  "myimage",
				Value: *v1.NewStructuredValues("bar", "foo"),
			}, {
				Name:  "FOO",
				Value: *v1.NewStructuredValues("hello", "world"),
			}},
		},
	}
	dp := []v1.ParamSpec{{
		Name:    "something",
		Default: v1.NewStructuredValues("mydefault", "mydefault2"),
	}, {
		Name:    "somethingelse",
		Default: v1.NewStructuredValues(""),
	}}
	want := applyMutation(simpleTaskSpec, func(spec *v1.TaskSpec) {
		spec.StepTemplate.Env[0].Value = "world"
		spec.StepTemplate.Image = "bar"

		spec.Steps[0].Image = "bar"
		spec.Steps[2].Image = "bar"
		spec.Steps[3].Image = ""

		spec.Steps[4].VolumeMounts[0].Name = "world"
		spec.Steps[4].VolumeMounts[0].SubPath = "sub/world/path"
		spec.Steps[4].VolumeMounts[0].MountPath = "path/to/world"
		spec.Steps[4].Image = "busybox:world"

		spec.Steps[5].Env[0].Value = "value-world"
		spec.Steps[5].Env[1].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = "config-world"
		spec.Steps[5].Env[1].ValueFrom.ConfigMapKeyRef.Key = "config-key-world"
		spec.Steps[5].Env[2].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "secret-world"
		spec.Steps[5].Env[2].ValueFrom.SecretKeyRef.Key = "secret-key-world"
		spec.Steps[5].EnvFrom[0].Prefix = "prefix-0-world"
		spec.Steps[5].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "config-world"
		spec.Steps[5].EnvFrom[1].Prefix = "prefix-1-world"
		spec.Steps[5].EnvFrom[1].SecretRef.LocalObjectReference.Name = "secret-world"
		spec.Steps[5].Image = "busybox:world"

		spec.Volumes[0].Name = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Key = "world"
		spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Path = "world"
		spec.Volumes[1].VolumeSource.Secret.SecretName = "world"
		spec.Volumes[1].VolumeSource.Secret.Items[0].Key = "world"
		spec.Volumes[1].VolumeSource.Secret.Items[0].Path = "world"
		spec.Volumes[2].VolumeSource.PersistentVolumeClaim.ClaimName = "world"
		spec.Volumes[3].VolumeSource.Projected.Sources[0].ConfigMap.Name = "world"
		spec.Volumes[3].VolumeSource.Projected.Sources[0].Secret.Name = "world"
		spec.Volumes[3].VolumeSource.Projected.Sources[0].ServiceAccountToken.Audience = "world"
		spec.Volumes[4].VolumeSource.CSI.VolumeAttributes["secretProviderClass"] = "world"
		spec.Volumes[4].VolumeSource.CSI.NodePublishSecretRef.Name = "world"

		spec.Sidecars[0].Image = "bar"
		spec.Sidecars[0].Env[0].Value = "world"
	})
	got := resources.ApplyParameters(context.Background(), simpleTaskSpecArrayIndexing, tr, dp...)
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
	}
}

func TestApplyObjectParameters(t *testing.T) {
	// define the taskrun to test values provided by taskrun can overwrite the values provided in spec's default
	tr := &v1.TaskRun{
		Spec: v1.TaskRunSpec{
			Params: []v1.Param{{
				Name: "myObject",
				Value: *v1.NewObject(map[string]string{
					"key1": "taskrun-value-for-key1",
					"key2": "taskrun-value-for-key2",
				}),
			}},
		},
	}
	dp := []v1.ParamSpec{{
		Name: "myObject",
		Default: v1.NewObject(map[string]string{
			"key1": "default-value-for-key1",
			"key2": "default-value-for-key2",
		}),
	}}

	want := applyMutation(objectParamTaskSpec, func(spec *v1.TaskSpec) {
		spec.Sidecars[0].Image = "taskrun-value-for-key1"
		spec.Sidecars[0].Env[0].Value = "taskrun-value-for-key2"

		spec.StepTemplate.Image = "taskrun-value-for-key1"
		spec.StepTemplate.Env[0].Value = "taskrun-value-for-key2"

		spec.Steps[0].Image = "taskrun-value-for-key1"
		spec.Steps[0].WorkingDir = "path/to/taskrun-value-for-key2"
		spec.Steps[0].Args = []string{"first taskrun-value-for-key1", "second taskrun-value-for-key2"}

		spec.Steps[0].VolumeMounts[0].Name = "taskrun-value-for-key1"
		spec.Steps[0].VolumeMounts[0].SubPath = "sub/taskrun-value-for-key2/path"
		spec.Steps[0].VolumeMounts[0].MountPath = "path/to/taskrun-value-for-key2"

		spec.Steps[0].Env[0].Value = "value-taskrun-value-for-key1"
		spec.Steps[0].Env[1].ValueFrom.ConfigMapKeyRef.LocalObjectReference.Name = "config-taskrun-value-for-key1"
		spec.Steps[0].Env[1].ValueFrom.ConfigMapKeyRef.Key = "config-key-taskrun-value-for-key2"
		spec.Steps[0].Env[2].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "secret-taskrun-value-for-key1"
		spec.Steps[0].Env[2].ValueFrom.SecretKeyRef.Key = "secret-key-taskrun-value-for-key2"
		spec.Steps[0].EnvFrom[0].Prefix = "prefix-0-taskrun-value-for-key1"
		spec.Steps[0].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "config-taskrun-value-for-key1"
		spec.Steps[0].EnvFrom[1].Prefix = "prefix-1-taskrun-value-for-key1"
		spec.Steps[0].EnvFrom[1].SecretRef.LocalObjectReference.Name = "secret-taskrun-value-for-key1"

		spec.Volumes[0].Name = "taskrun-value-for-key1"
		spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = "taskrun-value-for-key1"
		spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Key = "taskrun-value-for-key1"
		spec.Volumes[0].VolumeSource.ConfigMap.Items[0].Path = "taskrun-value-for-key2"
		spec.Volumes[0].VolumeSource.Secret.SecretName = "taskrun-value-for-key1"
		spec.Volumes[0].VolumeSource.Secret.Items[0].Key = "taskrun-value-for-key1"
		spec.Volumes[0].VolumeSource.Secret.Items[0].Path = "taskrun-value-for-key2"
		spec.Volumes[1].VolumeSource.PersistentVolumeClaim.ClaimName = "taskrun-value-for-key1"
		spec.Volumes[2].VolumeSource.Projected.Sources[0].ConfigMap.Name = "taskrun-value-for-key1"
		spec.Volumes[2].VolumeSource.Projected.Sources[0].Secret.Name = "taskrun-value-for-key1"
		spec.Volumes[2].VolumeSource.Projected.Sources[0].ServiceAccountToken.Audience = "taskrun-value-for-key2"
		spec.Volumes[3].VolumeSource.CSI.VolumeAttributes["secretProviderClass"] = "taskrun-value-for-key1"
		spec.Volumes[3].VolumeSource.CSI.NodePublishSecretRef.Name = "taskrun-value-for-key1"
	})
	got := resources.ApplyParameters(context.Background(), objectParamTaskSpec, tr, dp...)
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("ApplyParameters() got diff %s", diff.PrintWantGot(d))
	}
}

func TestApplyWorkspaces(t *testing.T) {
	names.TestingSeed()
	ts := &v1.TaskSpec{
		StepTemplate: &v1.StepTemplate{
			Env: []corev1.EnvVar{{
				Name:  "template-var",
				Value: "$(workspaces.myws.volume)",
			}, {
				Name:  "pvc-name",
				Value: "$(workspaces.myws.claim)",
			}, {
				Name:  "non-pvc-name",
				Value: "$(workspaces.otherws.claim)",
			}},
		},
		Steps: []v1.Step{{
			Name:       "$(workspaces.myws.volume)",
			Image:      "$(workspaces.otherws.volume)",
			WorkingDir: "$(workspaces.otherws.volume)",
			Args:       []string{"$(workspaces.myws.path)"},
		}, {
			Name:  "foo",
			Image: "bar",
			VolumeMounts: []corev1.VolumeMount{{
				Name:      "$(workspaces.myws.volume)",
				MountPath: "path/to/$(workspaces.otherws.path)",
				SubPath:   "$(workspaces.myws.volume)",
			}},
		}, {
			Name:  "foo",
			Image: "bar",
			Env: []corev1.EnvVar{{
				Name:  "foo",
				Value: "$(workspaces.myws.volume)",
			}, {
				Name: "baz",
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: "$(workspaces.myws.volume)"},
						Key:                  "$(workspaces.myws.volume)",
					},
				},
			}},
			EnvFrom: []corev1.EnvFromSource{{
				Prefix: "$(workspaces.myws.volume)",
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "$(workspaces.myws.volume)"},
				},
			}},
		}},
		Volumes: []corev1.Volume{{
			Name: "$(workspaces.myws.volume)",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: "$(workspaces.myws.volume)",
					},
				},
			}}, {
			Name: "some-secret",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "$(workspaces.myws.volume)",
				},
			}}, {
			Name: "some-pvc",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: "$(workspaces.myws.volume)",
				},
			},
		}},
	}
	for _, tc := range []struct {
		name  string
		spec  *v1.TaskSpec
		decls []v1.WorkspaceDeclaration
		binds []v1.WorkspaceBinding
		want  *v1.TaskSpec
	}{{
		name: "workspace-variable-replacement",
		spec: ts.DeepCopy(),
		decls: []v1.WorkspaceDeclaration{{
			Name: "myws",
		}, {
			Name:      "otherws",
			MountPath: "/foo",
		}},
		binds: []v1.WorkspaceBinding{{
			Name: "myws",
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: "foo",
			},
		}, {
			Name:     "otherws",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
		want: applyMutation(ts, func(spec *v1.TaskSpec) {
			spec.StepTemplate.Env[0].Value = "ws-9l9zj"
			spec.StepTemplate.Env[1].Value = "foo"
			spec.StepTemplate.Env[2].Value = ""

			spec.Steps[0].Name = "ws-9l9zj"
			spec.Steps[0].Image = "ws-mz4c7"
			spec.Steps[0].WorkingDir = "ws-mz4c7"
			spec.Steps[0].Args = []string{"/workspace/myws"}

			spec.Steps[1].VolumeMounts[0].Name = "ws-9l9zj"
			spec.Steps[1].VolumeMounts[0].MountPath = "path/to//foo"
			spec.Steps[1].VolumeMounts[0].SubPath = "ws-9l9zj"

			spec.Steps[2].Env[0].Value = "ws-9l9zj"
			spec.Steps[2].Env[1].ValueFrom.SecretKeyRef.LocalObjectReference.Name = "ws-9l9zj"
			spec.Steps[2].Env[1].ValueFrom.SecretKeyRef.Key = "ws-9l9zj"
			spec.Steps[2].EnvFrom[0].Prefix = "ws-9l9zj"
			spec.Steps[2].EnvFrom[0].ConfigMapRef.LocalObjectReference.Name = "ws-9l9zj"

			spec.Volumes[0].Name = "ws-9l9zj"
			spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = "ws-9l9zj"
			spec.Volumes[1].VolumeSource.Secret.SecretName = "ws-9l9zj"
			spec.Volumes[2].VolumeSource.PersistentVolumeClaim.ClaimName = "ws-9l9zj"
		}),
	}, {
		name: "optional-workspace-provided-variable-replacement",
		spec: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `test "$(workspaces.ows.bound)" = "true" && echo "$(workspaces.ows.path)"`,
		}}},
		decls: []v1.WorkspaceDeclaration{{
			Name:     "ows",
			Optional: true,
		}},
		binds: []v1.WorkspaceBinding{{
			Name:     "ows",
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}},
		want: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `test "true" = "true" && echo "/workspace/ows"`,
		}}},
	}, {
		name: "optional-workspace-omitted-variable-replacement",
		spec: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `test "$(workspaces.ows.bound)" = "true" && echo "$(workspaces.ows.path)"`,
		}}},
		decls: []v1.WorkspaceDeclaration{{
			Name:     "ows",
			Optional: true,
		}},
		binds: []v1.WorkspaceBinding{}, // intentionally omitted ows binding
		want: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `test "false" = "true" && echo ""`,
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			vols := workspace.CreateVolumes(tc.binds)
			got := resources.ApplyWorkspaces(context.Background(), tc.spec, tc.decls, tc.binds, vols)
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("TestApplyWorkspaces() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestApplyWorkspaces_IsolatedWorkspaces(t *testing.T) {
	for _, tc := range []struct {
		name  string
		spec  *v1.TaskSpec
		decls []v1.WorkspaceDeclaration
		binds []v1.WorkspaceBinding
		want  *v1.TaskSpec
	}{{
		name: "step-workspace-with-custom-mountpath",
		spec: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `echo "$(workspaces.ws.path)"`,
			Workspaces: []v1.WorkspaceUsage{{
				Name:      "ws",
				MountPath: "/foo",
			}},
		}, {
			Script: `echo "$(workspaces.ws.path)"`,
		}}, Sidecars: []v1.Sidecar{{
			Script: `echo "$(workspaces.ws.path)"`,
		}}},
		decls: []v1.WorkspaceDeclaration{{
			Name: "ws",
		}},
		want: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `echo "/foo"`,
			Workspaces: []v1.WorkspaceUsage{{
				Name:      "ws",
				MountPath: "/foo",
			}},
		}, {
			Script: `echo "/workspace/ws"`,
		}}, Sidecars: []v1.Sidecar{{
			Script: `echo "/workspace/ws"`,
		}}},
	}, {
		name: "sidecar-workspace-with-custom-mountpath",
		spec: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `echo "$(workspaces.ws.path)"`,
		}}, Sidecars: []v1.Sidecar{{
			Script: `echo "$(workspaces.ws.path)"`,
			Workspaces: []v1.WorkspaceUsage{{
				Name:      "ws",
				MountPath: "/bar",
			}},
		}}},
		decls: []v1.WorkspaceDeclaration{{
			Name: "ws",
		}},
		want: &v1.TaskSpec{Steps: []v1.Step{{
			Script: `echo "/workspace/ws"`,
		}}, Sidecars: []v1.Sidecar{{
			Script: `echo "/bar"`,
			Workspaces: []v1.WorkspaceUsage{{
				Name:      "ws",
				MountPath: "/bar",
			}},
		}}},
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := config.ToContext(context.Background(), &config.Config{
				FeatureFlags: &config.FeatureFlags{
					EnableAPIFields: "alpha",
				},
			})
			vols := workspace.CreateVolumes(tc.binds)
			got := resources.ApplyWorkspaces(ctx, tc.spec, tc.decls, tc.binds, vols)
			if d := cmp.Diff(tc.want, got); d != "" {
				t.Errorf("TestApplyWorkspaces() got diff %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestContext(t *testing.T) {
	for _, tc := range []struct {
		description string
		taskName    string
		tr          v1.TaskRun
		spec        v1.TaskSpec
		want        v1.TaskSpec
	}{{
		description: "context taskName replacement without taskRun in spec container",
		taskName:    "Task1",
		tr:          v1.TaskRun{},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.task.name)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "Task1-1",
			}},
		},
	}, {
		description: "context taskName replacement with taskRun in spec container",
		taskName:    "Task1",
		tr: v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunName",
			},
		},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.task.name)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "Task1-1",
			}},
		},
	}, {
		description: "context taskRunName replacement with defined taskRun in spec container",
		taskName:    "Task1",
		tr: v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name: "taskrunName",
			},
		},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.taskRun.name)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "taskrunName-1",
			}},
		},
	}, {
		description: "context taskRunName replacement with no defined taskRun name in spec container",
		taskName:    "Task1",
		tr:          v1.TaskRun{},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.taskRun.name)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "-1",
			}},
		},
	}, {
		description: "context taskRun namespace replacement with no defined namepsace in spec container",
		taskName:    "Task1",
		tr:          v1.TaskRun{},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.taskRun.namespace)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "-1",
			}},
		},
	}, {
		description: "context taskRun namespace replacement with defined namepsace in spec container",
		taskName:    "Task1",
		tr: v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "taskrunName",
				Namespace: "trNamespace",
			},
		},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.taskRun.namespace)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "trNamespace-1",
			}},
		},
	}, {
		description: "context taskRunName replacement with no defined taskName in spec container",
		tr:          v1.TaskRun{},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.task.name)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "-1",
			}},
		},
	}, {
		description: "context UID replacement",
		taskName:    "Task1",
		tr: v1.TaskRun{
			ObjectMeta: metav1.ObjectMeta{
				UID: "UID-1",
			},
		},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.taskRun.uid)",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "UID-1",
			}},
		},
	}, {
		description: "context retry count replacement",
		tr: v1.TaskRun{
			Status: v1.TaskRunStatus{
				TaskRunStatusFields: v1.TaskRunStatusFields{
					RetriesStatus: []v1.TaskRunStatus{{
						Status: duckv1.Status{
							Conditions: []apis.Condition{{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionFalse,
							}},
						},
					}, {
						Status: duckv1.Status{
							Conditions: []apis.Condition{{
								Type:   apis.ConditionSucceeded,
								Status: corev1.ConditionFalse,
							}},
						},
					}},
				},
			},
		},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.task.retry-count)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "2-1",
			}},
		},
	}, {
		description: "context retry count replacement with task that never retries",
		tr:          v1.TaskRun{},
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "$(context.task.retry-count)-1",
			}},
		},
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Name:  "ImageName",
				Image: "0-1",
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			got := resources.ApplyContexts(&tc.spec, tc.taskName, &tc.tr)
			if d := cmp.Diff(&tc.want, got); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskResults(t *testing.T) {
	names.TestingSeed()
	ts := &v1.TaskSpec{
		Results: []v1.TaskResult{{
			Name:        "current.date.unix.timestamp",
			Description: "The current date in unix timestamp format",
		}, {
			Name:        "current-date-human-readable",
			Description: "The current date in humand readable format"},
		},
		Steps: []v1.Step{{
			Name:   "print-date-unix-timestamp",
			Image:  "bash:latest",
			Args:   []string{"$(results[\"current.date.unix.timestamp\"].path)"},
			Script: "#!/usr/bin/env bash\ndate +%s | tee $(results[\"current.date.unix.timestamp\"].path)",
		}, {
			Name:   "print-date-human-readable",
			Image:  "bash:latest",
			Script: "#!/usr/bin/env bash\ndate | tee $(results.current-date-human-readable.path)",
		}, {
			Name:   "print-date-human-readable-again",
			Image:  "bash:latest",
			Script: "#!/usr/bin/env bash\ndate | tee $(results['current-date-human-readable'].path)",
		}},
	}
	want := applyMutation(ts, func(spec *v1.TaskSpec) {
		spec.Steps[0].Script = "#!/usr/bin/env bash\ndate +%s | tee /tekton/results/current.date.unix.timestamp"
		spec.Steps[0].Args[0] = "/tekton/results/current.date.unix.timestamp"
		spec.Steps[1].Script = "#!/usr/bin/env bash\ndate | tee /tekton/results/current-date-human-readable"
		spec.Steps[2].Script = "#!/usr/bin/env bash\ndate | tee /tekton/results/current-date-human-readable"
	})
	got := resources.ApplyTaskResults(ts)
	if d := cmp.Diff(want, got); d != "" {
		t.Errorf("ApplyTaskResults() got diff %s", diff.PrintWantGot(d))
	}
}

func TestApplyStepExitCodePath(t *testing.T) {
	names.TestingSeed()
	ts := &v1.TaskSpec{
		Steps: []v1.Step{{
			Image:  "bash:latest",
			Script: "#!/usr/bin/env bash\nexit 11",
		}, {
			Name:   "failing-step",
			Image:  "bash:latest",
			Script: "#!/usr/bin/env bash\ncat $(steps.step-unnamed-0.exitCode.path)",
		}, {
			Name:   "check-failing-step",
			Image:  "bash:latest",
			Script: "#!/usr/bin/env bash\ncat $(steps.step-failing-step.exitCode.path)",
		}},
	}
	expected := applyMutation(ts, func(spec *v1.TaskSpec) {
		spec.Steps[1].Script = "#!/usr/bin/env bash\ncat /tekton/steps/step-unnamed-0/exitCode"
		spec.Steps[2].Script = "#!/usr/bin/env bash\ncat /tekton/steps/step-failing-step/exitCode"
	})
	got := resources.ApplyStepExitCodePath(ts)
	if d := cmp.Diff(expected, got); d != "" {
		t.Errorf("ApplyStepExitCodePath() got diff %s", diff.PrintWantGot(d))
	}
}

func TestApplyCredentialsPath(t *testing.T) {
	for _, tc := range []struct {
		description string
		spec        v1.TaskSpec
		path        string
		want        v1.TaskSpec
	}{{
		description: "replacement in spec container",
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Command: []string{"cp"},
				Args:    []string{"-R", "$(credentials.path)/", "$HOME"},
			}},
		},
		path: "/tekton/creds",
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Command: []string{"cp"},
				Args:    []string{"-R", "/tekton/creds/", "$HOME"},
			}},
		},
	}, {
		description: "replacement in spec Script",
		spec: v1.TaskSpec{
			Steps: []v1.Step{{
				Script: `cp -R "$(credentials.path)/" $HOME`,
			}},
		},
		path: "/tekton/home",
		want: v1.TaskSpec{
			Steps: []v1.Step{{
				Script: `cp -R "/tekton/home/" $HOME`,
			}},
		},
	}} {
		t.Run(tc.description, func(t *testing.T) {
			got := resources.ApplyCredentialsPath(&tc.spec, tc.path)
			if d := cmp.Diff(&tc.want, got); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}
