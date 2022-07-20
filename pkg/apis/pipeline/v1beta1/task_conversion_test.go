/*
Copyright 2020 The Tetkon Authors

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

package v1beta1_test

import (
	"context"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskConversionBadType(t *testing.T) {
	good, bad := &v1beta1.Task{}, &v1beta1.Pipeline{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}
}

func TestTaskConversion(t *testing.T) {
	versions := []apis.Convertible{&v1.Task{}}

	tests := []struct {
		name string
		in   *v1beta1.Task
	}{{
		name: "simple task",
		in: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{
				Description: "test",
				Steps: []v1beta1.Step{{
					Image: "foo",
				}},
				Volumes: []corev1.Volume{{}},
				Params: []v1beta1.ParamSpec{{
					Name:        "param-1",
					Type:        v1beta1.ParamTypeString,
					Description: "My first param",
				}},
			},
		},
	}, {
		name: "task conversion all non deprecated fields",
		in: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{
				Description: "test",
				Steps: []v1beta1.Step{{
					Name:            "step",
					Image:           "foo",
					Command:         []string{"hello"},
					Args:            []string{"world"},
					WorkingDir:      "/dir",
					EnvFrom:         []corev1.EnvFromSource{{Prefix: "prefix"}},
					Env:             []corev1.EnvVar{{Name: "var"}},
					Resources:       corev1.ResourceRequirements{},
					VolumeMounts:    []corev1.VolumeMount{},
					VolumeDevices:   []corev1.VolumeDevice{},
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{},
					Script:          "echo hello",
					Timeout:         &metav1.Duration{Duration: time.Hour},
					Workspaces:      []v1beta1.WorkspaceUsage{{Name: "workspace"}},
					OnError:         "continue",
					StdoutConfig:    &v1beta1.StepOutputConfig{Path: "/path"},
					StderrConfig:    &v1beta1.StepOutputConfig{Path: "/another-path"},
				}},
				StepTemplate: &v1beta1.StepTemplate{
					Image:           "foo",
					Command:         []string{"hello"},
					Args:            []string{"world"},
					WorkingDir:      "/dir",
					EnvFrom:         []corev1.EnvFromSource{{Prefix: "prefix"}},
					Env:             []corev1.EnvVar{{Name: "var"}},
					Resources:       corev1.ResourceRequirements{},
					VolumeMounts:    []corev1.VolumeMount{},
					VolumeDevices:   []corev1.VolumeDevice{},
					ImagePullPolicy: corev1.PullIfNotPresent,
					SecurityContext: &corev1.SecurityContext{},
				},
				Sidecars: []v1beta1.Sidecar{{
					Name:                     "step",
					Image:                    "foo",
					Command:                  []string{"hello"},
					Args:                     []string{"world"},
					WorkingDir:               "/dir",
					Ports:                    []corev1.ContainerPort{},
					EnvFrom:                  []corev1.EnvFromSource{{Prefix: "prefix"}},
					Env:                      []corev1.EnvVar{{Name: "var"}},
					Resources:                corev1.ResourceRequirements{},
					VolumeMounts:             []corev1.VolumeMount{},
					VolumeDevices:            []corev1.VolumeDevice{},
					LivenessProbe:            &corev1.Probe{},
					ReadinessProbe:           &corev1.Probe{},
					StartupProbe:             &corev1.Probe{},
					Lifecycle:                &corev1.Lifecycle{},
					TerminationMessagePath:   "/path",
					TerminationMessagePolicy: corev1.TerminationMessageReadFile,
					ImagePullPolicy:          corev1.PullIfNotPresent,
					SecurityContext:          &corev1.SecurityContext{},
					Stdin:                    true,
					StdinOnce:                true,
					TTY:                      true,
					Script:                   "echo hello",
					Workspaces:               []v1beta1.WorkspaceUsage{{Name: "workspace"}},
				}},
				Volumes: []corev1.Volume{{Name: "volume"}},
				Params: []v1beta1.ParamSpec{{
					Name:        "param-1",
					Type:        v1beta1.ParamTypeString,
					Description: "My first param",
					Properties:  map[string]v1beta1.PropertySpec{"foo": {Type: v1beta1.ParamTypeString}},
					Default:     v1beta1.NewArrayOrString("bar"),
				}},
				Workspaces: []v1beta1.WorkspaceDeclaration{{
					Name:        "workspace",
					Description: "description",
					MountPath:   "/foo",
					ReadOnly:    true,
					Optional:    true,
				}},

				Results: []v1beta1.TaskResult{{
					Name:        "result",
					Type:        v1beta1.ResultsTypeObject,
					Properties:  map[string]v1beta1.PropertySpec{"property": {Type: v1beta1.ParamTypeString}},
					Description: "description",
				}},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
					return
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.Task{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.in, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}

func TestTaskConversionFromDeprecated(t *testing.T) {
	// TODO(#4546): We're just dropping Resources when converting from
	// v1beta1 to v1. Before moving the stored version to v1, we should
	// come up with a better strategy
	versions := []apis.Convertible{&v1.Task{}}
	tests := []struct {
		name string
		in   *v1beta1.Task
		want *v1beta1.Task
	}{{
		name: "input resources",
		in: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{},
				},
			},
		},
		want: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{},
		},
	}, {
		name: "output resources",
		in: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Outputs: []v1beta1.TaskResource{},
				},
			},
		},
		want: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{},
		},
	}}
	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}
				t.Logf("ConvertTo() = %#v", ver)
				got := &v1beta1.Task{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}
				t.Logf("ConvertFrom() = %#v", got)
				if d := cmp.Diff(test.want, got); d != "" {
					t.Errorf("roundtrip %s", diff.PrintWantGot(d))
				}
			})
		}
	}
}
