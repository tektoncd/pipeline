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

// CommentsTags maps marker prefixes to a set of tags containing keys and values
type CommentTags map[string]CommentTag

// CommentTags maps keys to a list of values
type CommentTag map[string][]string

// ExtractCommentTags parses comments for lines of the form:
//
//	 "marker" + "prefix" + ':' + "key=value,key2=value2".
//
//	In the following example the marker is '+' and the prefix is 'foo':
//	 +foo:key=value1,key2=value2,key=value3
//
// Values are optional; empty map is the default. A tag can be specified more than
// one time and all values are returned.  If the resulting map has an entry for
// a key, the value (a slice) is guaranteed to have at least 1 element.
//
// Example: if you pass "+" for 'marker', and the following lines are in
// the comments:
//
//	+foo:key=value1,key2=value2,key=value3
//	+bar
//
// Then this function will return:
//
//	 map[string]map[string]string{
//	   "foo":{
//	    "key":  []string{"value1", "value3"},
//	    "key2": []string{"value2"}
//	   },
//	   "bar": {},
//	}
//
// Users are not expected to repeat values.
func ExtractCommentTags(marker string, lines []string) CommentTags {
	out := CommentTags{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if len(line) == 0 || !strings.HasPrefix(line, marker) {
			continue
		}

		options := strings.SplitN(line[len(marker):], ":", 2)
		prefix := options[0]

		if len(options) == 2 {
			vals := strings.Split(options[1], ",")

			opts := out[prefix]
			if opts == nil {
				opts = make(CommentTag, len(vals))
				out[prefix] = opts
			}

			for _, pair := range vals {
				kv := strings.SplitN(pair, "=", 2)
				if len(kv) == 1 && kv[0] == "" {
					continue
				}
				if _, ok := opts[kv[0]]; !ok {
					opts[kv[0]] = []string{}
				}
				if len(kv) == 2 {
					opts[kv[0]] = append(opts[kv[0]], kv[1])
				}
			}
		} else if len(options) == 1 && options[0] != "" {
			if _, has := out[prefix]; !has {
				out[prefix] = CommentTag{}
			}
		}
	}
	return out
}
