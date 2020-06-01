/*
Copyright 2020 The Knative Authors.

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
package generators

import "strings"

// Adapted from the k8s.io comment parser https://github.com/kubernetes/gengo/blob/master/types/comments.go

// ExtractCommentTags parses comments for lines of the form:
//
//   'marker' + ':' "key=value,key2=value2".
//
// Values are optional; empty map is the default.  A tag can be specified more than
// one time and all values are returned.  If the resulting map has an entry for
// a key, the value (a slice) is guaranteed to have at least 1 element.
//
// Example: if you pass "+" for 'marker', and the following lines are in
// the comments:
//   +foo:key=value1,key2=value2
//   +bar
//
// Then this function will return:
//   map[string]map[string]string{"foo":{"key":value1","key2":"value2"}, "bar": nil}
//
// Users are not expected to repeat values.
func ExtractCommentTags(marker string, lines []string) map[string]map[string]string {
	out := map[string]map[string]string{}
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 || !strings.HasPrefix(line, marker) {
			continue
		}

		options := strings.SplitN(line[len(marker):], ":", 2)
		if len(options) == 2 {
			vals := strings.Split(options[1], ",")

			opts := out[options[0]]
			if opts == nil {
				opts = make(map[string]string, len(vals))
			}

			for _, pair := range vals {
				if kv := strings.SplitN(pair, "=", 2); len(kv) == 2 {
					opts[kv[0]] = kv[1]
				} else if kv[0] != "" {
					opts[kv[0]] = ""
				}
			}
			if len(opts) == 0 {
				out[options[0]] = nil
			} else {
				out[options[0]] = opts
			}
		} else if len(options) == 1 && options[0] != "" {
			if _, has := out[options[0]]; !has {
				out[options[0]] = nil
			}
		}
	}
	return out
}
