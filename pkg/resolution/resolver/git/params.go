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

const (
	// urlParam is the git repo url when using the anonymous/full clone approach
	urlParam string = "url"
	// orgParam is the organization to find the repository in when using the SCM API approach
	orgParam = "org"
	// repoParam is the repository to use when using the SCM API approach
	repoParam = "repo"
	// pathParam is the pathInRepo into the git repo where a file is located. This is used with both approaches.
	pathParam string = "pathInRepo"
	// revisionParam is the git revision that a file should be fetched from. This is used with both approaches.
	revisionParam string = "revision"
)
