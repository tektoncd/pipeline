// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pipelinerun

import (
	"sort"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func SortPipelineRunsByStartTime(prs []v1alpha1.PipelineRun) []v1alpha1.PipelineRun {
	sort.Slice(prs, func(i, j int) bool {
		if prs[j].Status.StartTime == nil {
			return false
		}

		if prs[i].Status.StartTime == nil {
			return true
		}
		return prs[j].Status.StartTime.Before(prs[i].Status.StartTime)
	})

	return prs
}
