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

package v1alpha2_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha2"
	"knative.dev/pkg/webhook"
)

func TestTypes(t *testing.T) {
	// Assert that types satisfy webhook interface.
	// var _ webhook.GenericCRD = (*v1alpha2.ClusterTask)(nil)
	// var _ webhook.GenericCRD = (*v1alpha2.TaskRun)(nil)
	// var _ webhook.GenericCRD = (*v1alpha2.PipelineResource)(nil)
	var _ webhook.GenericCRD = (*v1alpha2.Task)(nil)
	// var _ webhook.GenericCRD = (*v1alpha2.TaskRun)(nil)
	// var _ webhook.GenericCRD = (*v1alpha2.Condition)(nil)
}
