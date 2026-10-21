/*
Copyright 2024 The Tekton Authors
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

package hub

import "github.com/tektoncd/pipeline/pkg/resolution/resource"

// ParamURL is the parameter defining a custom hub API endpoint to use
// instead of the cluster-configured default. When specified, it overrides
// the ARTIFACT_HUB_API or TEKTON_HUB_API environment variable based on the
// resolution type.
const ParamURL = resource.ParamURL
