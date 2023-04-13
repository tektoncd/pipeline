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

package result

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestRunResult_UnmarshalJSON(t *testing.T) {
	testcases := []struct {
		name string
		data string
		pr   RunResult
	}{{
		name: "type defined as string - TaskRunResult",
		data: "{\"key\":\"resultName\",\"value\":\"resultValue\", \"type\": \"TaskRunResult\"}",
		pr:   RunResult{Key: "resultName", Value: "resultValue", ResultType: TaskRunResultType},
	},
		{
			name: "type defined as string - InternalTektonResult",
			data: "{\"key\":\"resultName\",\"value\":\"\", \"type\": \"InternalTektonResult\"}",
			pr:   RunResult{Key: "resultName", Value: "", ResultType: InternalTektonResultType},
		}, {
			name: "type defined as int",
			data: "{\"key\":\"resultName\",\"value\":\"\", \"type\": 1}",
			pr:   RunResult{Key: "resultName", Value: "", ResultType: TaskRunResultType},
		}}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			pipRes := RunResult{}
			if err := json.Unmarshal([]byte(tc.data), &pipRes); err != nil {
				t.Errorf("Unexpected error when unmarshalling the json into RunResult")
			}
			if d := cmp.Diff(tc.pr, pipRes); d != "" {
				t.Errorf(diff.PrintWantGot(d))
			}
		})
	}
}
