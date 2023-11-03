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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
)

func TestTaskConversionBadType(t *testing.T) {
	good, bad := &v1beta1.Task{}, &v1beta1.Pipeline{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", bad)
	}
}

func TestTaskConversion(t *testing.T) {
	simpleTaskYAML := `
metadata:
  name: foo
  namespace: bar
  generation: 1
spec:
  displayName: "task-display-name"
  description: test
  steps:
  - image: foo
  params:
  - name: param-1
    type: string
    enum: ["v1", "v2"]
    description: my first param
  results:
  - name: result-1
    type: string
    description: a result
`
	multiStepTaskYAML := `
metadata:
  name: foo
  namespace: bar
  generation: 1
spec:
  displayName: "task-display-name"
  description: test
  steps:
  - image: foo
  - image: bar
`
	stepActionTaskYAML := `
metadata:
  name: foo
  namespace: bar
spec:
  steps:
    - ref:
        name: "step-action"
      params:
        - name: param1
          value: hello
`

	remoteStepActionTaskYAML := `
metadata:
  name: foo
  namespace: bar
spec:
  steps:
    - ref:
        resolver: "git"
        params:
          - name: url
            value: https://github.com/tektoncd/catalog.git
          - name: pathInRepo
            value: /stepaction/sample/sample.yaml
          - name: revision
            value: main
`

	taskWithAllNoDeprecatedFieldsYAML := `
metadata:
  name: foo
  namespace: bar
  generation: 1
spec:
  displayName: "task-display-name"
  description: test
  steps:
  - name: step
    image: foo
    command: ["hello"]
    args: ["world"]
    workingDir: "/dir"
    envFrom:
    - prefix: prefix
    env:
    - name: var
    resources:
      limits:
    volumeMounts:
    volumeDevices:
    imagePullPolicy: IfNotPresent
    securityContext:
      privileged: true
    script: "echo 'hello world'"
    timeout: 1h
    workspaces:
    - name: workspace
    onError: continue
    stdoutConfig:
      path: /path
    stderrConfig:
      path: /another-path
  stepTemplate:
    image: foo
    command: ["hello"]
    args: ["world"]
    workingDir: "/dir"
    envFrom:
    - prefix: prefix
    env:
    - name: var
    resources:
      limits:
    volumeMounts:
    volumeDevices:
    imagePullPolicy: IfNotPresent
    securityContext:
      privileged: true
  sidecars:
  - name: sidecar
    image: foo
    command: ["hello"]
    args: ["world"]
    workingDir: "/dir"
    envFrom:
    - prefix: prefix
    env:
    - name: var
    resources:
      limits:
    volumeMounts:
    volumeDevices:
    imagePullPolicy: IfNotPresent
    securityContext:
      privileged: true
    script: "echo 'hello world'"
    timeout: 1h
    workspaces:
    - name: workspace
    onError: continue
    stdoutConfig:
      path: /path
    stderrConfig:
      path: /another-path
  volumes:
  - name: volume
  params:
  - name: param-1
    type: string
    description: my first param
    properties:
      foo: {type: string}
    default:
      type: string
      stringVal: bar
  workspaces:
  - name: workspace
    description: a workspace
    mountPath: /foo
    readOnly: true
    optional: true
  results:
  - name: result
    type: object
    properties:
      property: {type: string}
    description: description
`

	taskWithDeprecatedFieldsV1beta1YAML := `
metadata:
  name: foo
  namespace: bar
  generation: 1
spec:
  displayName: "task-display-name"
  description: test
  steps:
  - name: step-1
    ports:
    - name: port
    livenessProbe:
      initialDelaySeconds: 1
    readinessProbe:
      initialDelaySeconds: 2
    startupProbe:
      initialDelaySeconds: 3
    lifecycle:
      postStart:
        exec:
          command:
          - "lifecycle command"
    terminationMessagePath: path
    terminationMessagePolicy: policy
    stdin: true
    stdinOnce: true
    tty: true
  stepTemplate:
    image: foo
    ports:
    - name: port
    livenessProbe:
      initialDelaySeconds: 1
    readinessProbe:
      initialDelaySeconds: 2
    startupProbe:
      initialDelaySeconds: 3
    lifecycle:
      postStart:
        exec:
          command:
          - "lifecycle command"
    terminationMessagePath: path
    terminationMessagePolicy: policy
    stdin: true
    stdinOnce: true
    tty: true
`
	taskWithDeprecatedFieldsV1YAML := `
metadata:
  name: foo
  namespace: bar
  generation: 1
spec:
  displayName: "task-display-name"
  description: test
  steps:
  - name: step-1
  stepTemplate:
    image: foo
`
	taskWithoutStepTemplateYAML := `
metadata:
  name: foo
  namespace: bar
  generation: 1
spec:
  steps:
  - image: alpine
    name: echo
    readinessProbe:
      exec:
        command:
        - cat
        - /tmp/healthy
    resources: {}
    script: |
      echo "Good Morning!"
`
	simpleTaskV1beta1 := parse.MustParseV1beta1Task(t, simpleTaskYAML)
	simpleTaskV1 := parse.MustParseV1Task(t, simpleTaskYAML)

	multiStepTaskV1beta1 := parse.MustParseV1beta1Task(t, multiStepTaskYAML)
	multiStepTaskV1 := parse.MustParseV1Task(t, multiStepTaskYAML)

	stepActionTaskV1beta1 := parse.MustParseV1beta1Task(t, stepActionTaskYAML)
	stepActionTaskV1 := parse.MustParseV1Task(t, stepActionTaskYAML)

	remoteStepActionTaskV1beta1 := parse.MustParseV1beta1Task(t, remoteStepActionTaskYAML)
	remoteStepActionTaskV1 := parse.MustParseV1Task(t, remoteStepActionTaskYAML)

	taskWithAllNoDeprecatedFieldsV1beta1 := parse.MustParseV1beta1Task(t, taskWithAllNoDeprecatedFieldsYAML)
	taskWithAllNoDeprecatedFieldsV1 := parse.MustParseV1Task(t, taskWithAllNoDeprecatedFieldsYAML)

	taskWithDeprecatedFieldsV1beta1 := parse.MustParseV1beta1Task(t, taskWithDeprecatedFieldsV1beta1YAML)
	taskWithDeprecatedFieldsV1 := parse.MustParseV1Task(t, taskWithDeprecatedFieldsV1YAML)
	taskWithDeprecatedFieldsV1.ObjectMeta.Annotations = map[string]string{
		v1beta1.TaskDeprecationsAnnotationKey: `{"foo":{"deprecatedSteps":` +
			`[{"name":"","ports":[{"name":"port","containerPort":0}],"resources":{},"livenessProbe":{"initialDelaySeconds":1},"readinessProbe":{"initialDelaySeconds":2},"startupProbe":{"initialDelaySeconds":3},"lifecycle":{"postStart":{"exec":{"command":["lifecycle command"]}}},"terminationMessagePath":"path","terminationMessagePolicy":"policy","stdin":true,"stdinOnce":true,"tty":true}],` +
			`"deprecatedStepTemplate":{"name":"","ports":[{"name":"port","containerPort":0}],"resources":{},"livenessProbe":{"initialDelaySeconds":1},"readinessProbe":{"initialDelaySeconds":2},"startupProbe":{"initialDelaySeconds":3},"lifecycle":{"postStart":{"exec":{"command":["lifecycle command"]}}},"terminationMessagePath":"path","terminationMessagePolicy":"policy","stdin":true,"stdinOnce":true,"tty":true}}}`,
	}
	taskWithoutStepTemplateYAMLV1beta1 := parse.MustParseV1beta1Task(t, taskWithoutStepTemplateYAML)
	taskWithoutStepTemplateYAMLV1 := parse.MustParseV1Task(t, taskWithoutStepTemplateYAML)
	taskWithoutStepTemplateYAMLV1.ObjectMeta.Annotations = map[string]string{
		v1beta1.TaskDeprecationsAnnotationKey: `{"foo":{"deprecatedSteps":[{"name":"","resources":{},"readinessProbe":{"exec":{"command":["cat","/tmp/healthy"]}}}]}}`,
	}

	tests := []struct {
		name        string
		v1beta1Task *v1beta1.Task
		v1Task      *v1.Task
	}{{
		name:        "simple task",
		v1beta1Task: simpleTaskV1beta1,
		v1Task:      simpleTaskV1,
	}, {
		name:        "multi-steps task",
		v1beta1Task: multiStepTaskV1beta1,
		v1Task:      multiStepTaskV1,
	}, {
		name:        "task conversion all non deprecated fields",
		v1beta1Task: taskWithAllNoDeprecatedFieldsV1beta1,
		v1Task:      taskWithAllNoDeprecatedFieldsV1,
	}, {
		name:        "step action in task",
		v1beta1Task: stepActionTaskV1beta1,
		v1Task:      stepActionTaskV1,
	}, {
		name:        "remote step action in task",
		v1beta1Task: remoteStepActionTaskV1beta1,
		v1Task:      remoteStepActionTaskV1,
	}, {
		name:        "task conversion deprecated fields",
		v1beta1Task: taskWithDeprecatedFieldsV1beta1,
		v1Task:      taskWithDeprecatedFieldsV1,
	}, {
		name:        "task conversion deprecated step template fields panic check",
		v1beta1Task: taskWithoutStepTemplateYAMLV1beta1,
		v1Task:      taskWithoutStepTemplateYAMLV1,
	},
	}
	var ignoreTypeMeta = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v1Task := &v1.Task{}
			if err := test.v1beta1Task.ConvertTo(context.Background(), v1Task); err != nil {
				t.Errorf("ConvertTo() = %v", err)
				return
			}
			t.Logf("ConvertTo() = %#v", v1Task)
			if d := cmp.Diff(test.v1Task, v1Task, ignoreTypeMeta); d != "" {
				t.Errorf("expected v1Task is different from what's converted: %s", d)
			}
			gotV1beta1 := &v1beta1.Task{}
			if err := gotV1beta1.ConvertFrom(context.Background(), v1Task); err != nil {
				t.Errorf("ConvertFrom() = %v", err)
			}
			t.Logf("ConvertFrom() = %#v", gotV1beta1)
			if d := cmp.Diff(test.v1beta1Task, gotV1beta1, ignoreTypeMeta); d != "" {
				t.Errorf("roundtrip %s", diff.PrintWantGot(d))
			}
		})
	}
}

func TestTaskConversionFromDeprecated(t *testing.T) {
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
					Inputs: []v1beta1.TaskResource{{v1beta1.ResourceDeclaration{Name: "input-resource"}}},
				},
			},
		},
		want: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{
				Resources: &v1beta1.TaskResources{
					Inputs: []v1beta1.TaskResource{{v1beta1.ResourceDeclaration{Name: "input-resource"}}},
				},
			},
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
					Outputs: []v1beta1.TaskResource{{v1beta1.ResourceDeclaration{Name: "output-resource"}}},
				},
			},
		},
		want: &v1beta1.Task{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "foo",
				Namespace:  "bar",
				Generation: 1,
			},
			Spec: v1beta1.TaskSpec{Resources: &v1beta1.TaskResources{
				Outputs: []v1beta1.TaskResource{{v1beta1.ResourceDeclaration{Name: "output-resource"}}},
			}},
		},
	}}
	for _, test := range tests {
		versions := []apis.Convertible{&v1.Task{}}
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
