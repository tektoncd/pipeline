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

package builder

import (
	"time"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	tb "github.com/tektoncd/pipeline/test/builder"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TaskRunCompletionTime sets the completion time of the taskrun
func TaskRunCompletionTime(ct time.Time) tb.TaskRunStatusOp {
	return func(s *v1alpha1.TaskRunStatus) {
		s.CompletionTime = &metav1.Time{Time: ct}
	}
}

// TaskRunCreationTime sets the creation time of the taskrun
func TaskRunCreationTime(ct time.Time) tb.TaskRunOp {
	return func(t *v1alpha1.TaskRun) {
		t.CreationTimestamp = metav1.Time{Time: ct}
	}
}

// StepName adds a state to stepstate of TaskRunStatus.
func StepName(name string) tb.StepStateOp {
	return func(s *v1alpha1.StepState) {
		s.Name = name
	}
}
