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

package v1beta1

import "path/filepath"

// InputResourcePath returns the path where the given input resource
// will get mounted in a Pod
func InputResourcePath(r ResourceDeclaration) string {
	return path("/workspace", r)
}

// OutputResourcePath returns the path to the output resouce in a Pod
func OutputResourcePath(r ResourceDeclaration) string {
	return path("/workspace/output", r)
}

func path(root string, r ResourceDeclaration) string {
	if r.TargetPath != "" {
		if filepath.IsAbs(r.TargetPath) {
			return r.TargetPath
		}
		return filepath.Join("/workspace", r.TargetPath)
	}
	return filepath.Join(root, r.Name)
}
