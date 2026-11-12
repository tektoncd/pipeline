/*
Copyright 2025 The Tekton Authors

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

package substitution

import replacement "github.com/tektoncd/pipeline/internal/substitution"

// ApplyReplacements returns a string with references to parameters replaced,
// based on the mapping provided in replacements.
// For example, if the input string is "foo: $(params.foo)", and replacements maps "params.foo" to "bar",
// the output would be "foo: bar".
func ApplyReplacements(in string, replacements map[string]string) string {
	return replacement.ApplyReplacements(in, replacements)
}

// ApplyArrayReplacements takes an input string, and output an array of strings related to possible arrayReplacements. If there aren't any
// areas where the input can be split up via arrayReplacements, then just return an array with a single element,
// which is ApplyReplacements(in, replacements).
func ApplyArrayReplacements(in string, stringReplacements map[string]string, arrayReplacements map[string][]string) []string {
	return replacement.ApplyArrayReplacements(in, stringReplacements, arrayReplacements)
}
