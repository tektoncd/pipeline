package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/test/diff"
	"github.com/tektoncd/pipeline/test/parse"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPipelineConversionBadType(t *testing.T) {
	good, bad := &v1alpha1.StepAction{}, &v1beta1.Task{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

func TestStepActionConversion(t *testing.T) {
	stepActionWithAllFieldsYaml := `
metadata:
  name: foo
  namespace: bar
spec:
  name: all-fields
  image: foo
  command: ["hello"]
  args: ["world"]
  results:
    - name: result1
    - name: result2
  script: |
    echo "I am a Step Action!!!" >> $(step.results.result1.path)
    echo "I am a hidden step action!!!" >> $(step.results.result2.path)
  workingDir: "/dir"
  envFrom:
  - prefix: prefix
  params:
    - name: string-param
      type: string
      default: "a string param"
    - name: array-param
      type: array
      default:
        - an
        - array
        - param
    - name: object-param
      type: object
      properties:
        key1:
          type: string
        key2:
          type: string
        key3:
          type: string
      default:
        key1: "step-action default key1"
        key2: "step-action default key2"
        key3: "step-action default key3"
  volumeMounts:
    - name: data
      mountPath: /data
  securityContext:
    privileged: true
`

	stepActionV1alpha1 := parse.MustParseV1alpha1StepAction(t, stepActionWithAllFieldsYaml)
	stepActionV1beta1 := parse.MustParseV1beta1StepAction(t, stepActionWithAllFieldsYaml)

	var ignoreTypeMeta = cmpopts.IgnoreFields(metav1.TypeMeta{}, "Kind", "APIVersion")

	tests := []struct {
		name              string
		v1AlphaStepAction *v1alpha1.StepAction
		v1Beta1StepAction *v1beta1.StepAction
	}{{
		name:              "stepAction conversion with all fields",
		v1AlphaStepAction: stepActionV1alpha1,
		v1Beta1StepAction: stepActionV1beta1,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			v1Beta1StepAction := &v1beta1.StepAction{}
			if err := test.v1AlphaStepAction.ConvertTo(context.Background(), v1Beta1StepAction); err != nil {
				t.Errorf("ConvertTo() = %v", err)
				return
			}
			t.Logf("ConvertTo() = %#v", v1Beta1StepAction)
			if d := cmp.Diff(test.v1Beta1StepAction, v1Beta1StepAction, ignoreTypeMeta); d != "" {
				t.Errorf("expected v1Task is different from what's converted: %s", d)
			}
			gotV1alpha1 := &v1alpha1.StepAction{}
			if err := gotV1alpha1.ConvertFrom(context.Background(), v1Beta1StepAction); err != nil {
				t.Errorf("ConvertFrom() = %v", err)
			}
			t.Logf("ConvertFrom() = %#v", gotV1alpha1)
			if d := cmp.Diff(test.v1AlphaStepAction, gotV1alpha1, ignoreTypeMeta); d != "" {
				t.Errorf("roundtrip %s", diff.PrintWantGot(d))
			}
		})
	}
}
