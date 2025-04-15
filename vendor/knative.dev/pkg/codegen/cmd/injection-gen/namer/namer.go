/*
Copyright 2016 The Kubernetes Authors

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

package namer

import (
	"strings"

	"k8s.io/gengo/v2"
	"k8s.io/gengo/v2/namer"
	"k8s.io/gengo/v2/types"
)

// TagOverrideNamer is a namer which pulls names from a given tag, if specified,
// and otherwise falls back to a different namer.
type tagOverrideNamer struct {
	tagName  string
	fallback namer.Namer
}

// Name returns the tag value if it exists. It no tag was found the fallback namer will be used
func (n *tagOverrideNamer) Name(t *types.Type) string {
	nameOverride := extractTag(n.tagName, append(t.SecondClosestCommentLines, t.CommentLines...))
	if nameOverride != "" {
		return nameOverride
	}

	return n.fallback.Name(t)
}

// NewTagOverrideNamer creates a namer.Namer which uses the contents of the given tag as
// the name, or falls back to another Namer if the tag is not present.
func newTagOverrideNamer(tagName string, fallback namer.Namer) namer.Namer {
	return &tagOverrideNamer{
		tagName:  tagName,
		fallback: fallback,
	}
}

// extractTag gets the comment-tags for the key.  If the tag did not exist, it
// returns the empty string.
func extractTag(key string, lines []string) string {
	val, present := gengo.ExtractCommentTags("+", lines)[key]
	if !present || len(val) < 1 {
		return ""
	}

	return val[0]
}

type versionedClientsetNamer struct {
	public *ExceptionNamer
}

func (r *versionedClientsetNamer) Name(t *types.Type) string {
	// Turns type into a GroupVersion type string based on package.
	parts := strings.Split(t.Name.Package, "/")
	group := parts[len(parts)-2]
	version := parts[len(parts)-1]

	g := r.public.Name(&types.Type{Name: types.Name{Name: group, Package: t.Name.Package}})
	v := r.public.Name(&types.Type{Name: types.Name{Name: version, Package: t.Name.Package}})

	return g + v
}

// ExceptionNamer allows you specify exceptional cases with exact names.  This allows you to have control
// for handling various conflicts, like group and resource names for instance.
type ExceptionNamer struct {
	Exceptions map[string]string
	KeyFunc    func(*types.Type) string

	Delegate namer.Namer
}

// Name provides the requested name for a type.
func (n *ExceptionNamer) Name(t *types.Type) string {
	key := n.KeyFunc(t)
	if exception, ok := n.Exceptions[key]; ok {
		return exception
	}
	return n.Delegate.Name(t)
}

// lowercaseSingularNamer implements Namer
type lowercaseSingularNamer struct{}

// Name returns t's name in all lowercase.
func (n *lowercaseSingularNamer) Name(t *types.Type) string {
	return strings.ToLower(t.Name.Name)
}
