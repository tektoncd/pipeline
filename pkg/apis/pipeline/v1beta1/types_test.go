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

package v1beta1_test

import (
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/webhook/resourcesemantics"
)

func TestTypes(t *testing.T) {
	// Assert that types satisfy webhook interface.
	var _ resourcesemantics.GenericCRD = (*v1beta1.TaskRun)(nil)
	var _ resourcesemantics.GenericCRD = (*v1beta1.Task)(nil)
	var _ resourcesemantics.GenericCRD = (*v1beta1.Pipeline)(nil)
	// var _ resourcesemantics.GenericCRD = (*v1beta1.Condition)(nil)
}
