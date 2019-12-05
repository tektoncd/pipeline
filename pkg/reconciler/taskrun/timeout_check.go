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
	"time"

	apisconfig "github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

func CheckTimeout(tr *v1alpha1.TaskRun) bool {
	// If tr has not started, startTime should be zero.
	if tr.Status.StartTime.IsZero() {
		return false
	}

	timeout := tr.Spec.Timeout.Duration
	// If timeout is set to 0 or defaulted to 0, there is no timeout.
	if timeout == apisconfig.NoTimeoutDuration {
		return false
	}
	runtime := time.Since(tr.Status.StartTime.Time)
	return runtime > timeout
}
