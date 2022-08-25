/*
Copyright 2022 The Tekton Authors
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

package resolution

import (
	resolution "github.com/tektoncd/pipeline/pkg/resolution/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
)

var _ resolution.Request = &resolutionRequest{}
var _ resolution.OwnedRequest = &resolutionRequest{}

type resolutionRequest struct {
	resolution.Request
	owner kmeta.OwnerRefable
}

func (req *resolutionRequest) OwnerRef() metav1.OwnerReference {
	return *kmeta.NewControllerRef(req.owner)
}
