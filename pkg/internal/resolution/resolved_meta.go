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
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	"github.com/tektoncd/pipeline/pkg/trustedresources"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ResolvedObjectMeta contains both ObjectMeta and the metadata that identifies the source where the resource came from.
type ResolvedObjectMeta struct {
	*metav1.ObjectMeta `json:",omitempty"`
	// RefSource identifies where the spec came from.
	RefSource *v1.RefSource `json:",omitempty"`
	// VerificationResult contains the result of trusted resources verification
	VerificationResult *trustedresources.VerificationResult `json:",omitempty"`
}
