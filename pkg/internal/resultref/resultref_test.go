/*
Copyright 2023 The Tekton Authors

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

package resultref_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/internal/resultref"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestParseStepExpression(t *testing.T) {
	idx := 1000
	for _, tt := range []struct {
		name             string
		param            v1.Param
		wantParsedResult resultref.ParsedResult
		wantError        error
	}{{
		name: "a string step result ref",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(steps.sumSteps.results.sumResult)"),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumSteps",
			ResultName:   "sumResult",
			ResultType:   "string",
		},
		wantError: nil,
	}, {
		name: "an array step result ref with idx",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues(fmt.Sprintf("$(steps.sumSteps.results.sumResult[%v])", idx)),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumSteps",
			ResultName:   "sumResult",
			ResultType:   "array",
			ArrayIdx:     &idx,
		},
		wantError: nil,
	}, {
		name: "an array step result ref with [*]",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(steps.sumSteps.results.sumResult[*])"),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumSteps",
			ResultName:   "sumResult",
			ResultType:   "array",
		},
		wantError: nil,
	}, {
		name: "an object step result ref with attribute",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(steps.sumSteps.results.sumResult.sum)"),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumSteps",
			ResultName:   "sumResult",
			ResultType:   "object",
			ObjectKey:    "sum",
		},
		wantError: nil,
	}, {
		name: "an invalid step result ref",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(steps.sumSteps.results.sumResult.foo.bar)"),
		},
		wantParsedResult: resultref.ParsedResult{},
		wantError:        errors.New("must be one of the form 1). \"tasks.<taskName>.results.<resultName>\"; 2). \"tasks.<taskName>.results.<objectResultName>.<individualAttribute>\"; 3). \"steps.<stepName>.results.<resultName>\"; 4). \"steps.<stepName>.results.<objectResultName>.<individualAttribute>\""),
	}, {
		name: "not a step result ref",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(tasks.sumSteps.results.sumResult.foo.bar)"),
		},
		wantParsedResult: resultref.ParsedResult{},
		wantError:        errors.New("must be one of the form 1). \"steps.<stepName>.results.<resultName>\"; 2). \"steps.<stepName>.results.<objectResultName>.<individualAttribute>\""),
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, _ := tt.param.GetVarSubstitutionExpressions()
			gotParsedResult, gotError := resultref.ParseStepExpression(expressions[0])
			if d := cmp.Diff(tt.wantParsedResult, gotParsedResult); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if tt.wantError == nil {
				if gotError != nil {
					t.Errorf("ParseStepExpression() = %v, want nil", gotError)
				}
			} else {
				if gotError.Error() != tt.wantError.Error() {
					t.Errorf("ParseStepExpression() = \n%v\n, want \n%v", gotError.Error(), tt.wantError.Error())
				}
			}
		})
	}
}

func TestParseTaskExpression(t *testing.T) {
	idx := 1000
	for _, tt := range []struct {
		name             string
		param            v1.Param
		wantParsedResult resultref.ParsedResult
		wantError        error
	}{{
		name: "a string task result ref",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(tasks.sumTasks.results.sumResult)"),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumTasks",
			ResultName:   "sumResult",
			ResultType:   "string",
		},
		wantError: nil,
	}, {
		name: "an array task result ref with idx",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues(fmt.Sprintf("$(tasks.sumTasks.results.sumResult[%v])", idx)),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumTasks",
			ResultName:   "sumResult",
			ResultType:   "array",
			ArrayIdx:     &idx,
		},
		wantError: nil,
	}, {
		name: "an array task result ref with [*]",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(tasks.sumTasks.results.sumResult[*])"),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumTasks",
			ResultName:   "sumResult",
			ResultType:   "array",
		},
		wantError: nil,
	}, {
		name: "an object task result ref with attribute",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(tasks.sumTasks.results.sumResult.sum)"),
		},
		wantParsedResult: resultref.ParsedResult{
			ResourceName: "sumTasks",
			ResultName:   "sumResult",
			ResultType:   "object",
			ObjectKey:    "sum",
		},
		wantError: nil,
	}, {
		name: "an invalid task result ref",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(tasks.sumTasks.results.sumResult.foo.bar)"),
		},
		wantParsedResult: resultref.ParsedResult{},
		wantError:        errors.New("must be one of the form 1). \"tasks.<taskName>.results.<resultName>\"; 2). \"tasks.<taskName>.results.<objectResultName>.<individualAttribute>\"; 3). \"steps.<stepName>.results.<resultName>\"; 4). \"steps.<stepName>.results.<objectResultName>.<individualAttribute>\""),
	}, {
		name: "not a task result ref",
		param: v1.Param{
			Name:  "param",
			Value: *v1.NewStructuredValues("$(nottasks.sumTasks.results.sumResult.foo.bar)"),
		},
		wantParsedResult: resultref.ParsedResult{},
		wantError:        errors.New("must be one of the form 1). \"tasks.<taskName>.results.<resultName>\"; 2). \"tasks.<taskName>.results.<objectResultName>.<individualAttribute>\""),
	}} {
		t.Run(tt.name, func(t *testing.T) {
			expressions, _ := tt.param.GetVarSubstitutionExpressions()
			gotParsedResult, gotError := resultref.ParseTaskExpression(expressions[0])
			if d := cmp.Diff(tt.wantParsedResult, gotParsedResult); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
			if tt.wantError == nil {
				if gotError != nil {
					t.Errorf("ParseStepExpression() = %v, want nil", gotError)
				}
			} else {
				if gotError.Error() != tt.wantError.Error() {
					t.Errorf("ParseStepExpression() = \n%v\n, want \n%v", gotError.Error(), tt.wantError.Error())
				}
			}
		})
	}
}

func TestParseResultName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "array indexing",
			input: "anArrayResult[1]",
			want:  []string{"anArrayResult", "1"},
		},
		{
			name:  "array star reference",
			input: "anArrayResult[*]",
			want:  []string{"anArrayResult", "*"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resultName, idx := v1.ParseResultName(tt.input)
			if d := cmp.Diff(tt.want, []string{resultName, idx}); d != "" {
				t.Error(diff.PrintWantGot(d))
			}
		})
	}
}
