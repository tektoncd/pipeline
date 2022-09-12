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

package git

import "github.com/tektoncd/pipeline/pkg/apis/resolution"

var (
	// AnnotationKeyRevision is the commit hash that was fetched
	// from git
	AnnotationKeyRevision = resolution.GroupName + "/revision"

	// AnnotationKeyOrg is the org used
	AnnotationKeyOrg = resolution.GroupName + "/org"
	// AnnotationKeyRepo is the repo used
	AnnotationKeyRepo = resolution.GroupName + "/repo"
	// AnnotationKeyPath is the path used
	AnnotationKeyPath = resolution.GroupName + "/path"
	// AnnotationKeyURL is the repo URL used
	AnnotationKeyURL = resolution.GroupName + "/url"
)
