/*
Copyright 2020 The Knative Authors

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

package tags

import (
	"strings"

	"k8s.io/code-generator/cmd/client-gen/generators/util"
	"k8s.io/gengo/v2/types"
	"k8s.io/klog/v2"

	"knative.dev/pkg/codegen/cmd/injection-gen/args"
)

// Tags represents a genclient configuration for a single type.
type Tags struct {
	util.Tags

	GenerateDuck       bool
	GenerateReconciler bool
}

func (t Tags) NeedsInformerInjection() bool {
	return t.GenerateClient && !t.NoVerbs && t.HasVerb("list") && t.HasVerb("watch")
}

func (t Tags) NeedsDuckInjection() bool {
	return t.GenerateDuck
}

func (t Tags) NeedsReconciler(kind *types.Type, args *args.Args) bool {
	// Overrides
	kinds := strings.Split(args.ForceKinds, ",")
	for _, k := range kinds {
		if kind.Name.Name == k {
			klog.V(5).Infof("Kind %s was forced to generate reconciler.", k)
			return true
		}
	}
	// Normal
	return t.GenerateReconciler
}

// MustParseClientGenTags calls ParseClientGenTags but instead of returning error it panics.
func MustParseClientGenTags(lines []string) Tags {
	ret := Tags{
		Tags: util.MustParseClientGenTags(lines),
	}

	values := ExtractCommentTags("+", lines)

	_, ret.GenerateDuck = values["genduck"]

	_, ret.GenerateReconciler = values["genreconciler"]

	return ret
}
