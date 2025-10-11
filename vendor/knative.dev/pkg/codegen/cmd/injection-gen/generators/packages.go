/*
Copyright 2019 The Knative Authors.

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

import (
	"strings"

	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/v2"
	"k8s.io/gengo/v2/generator"
	"k8s.io/gengo/v2/namer"
	"k8s.io/gengo/v2/types"

	"knative.dev/pkg/codegen/cmd/injection-gen/args"
	"knative.dev/pkg/codegen/cmd/injection-gen/generators/client"
	"knative.dev/pkg/codegen/cmd/injection-gen/generators/duck"
	"knative.dev/pkg/codegen/cmd/injection-gen/generators/factory"
	"knative.dev/pkg/codegen/cmd/injection-gen/generators/informers"
	"knative.dev/pkg/codegen/cmd/injection-gen/generators/reconciler"
	"knative.dev/pkg/codegen/cmd/injection-gen/tags"
)

type genFunc func(*args.Args, string, clientgentypes.GroupVersion, string, []*types.Type) []generator.Target

// Packages makes the client package definition.
func Targets(context *generator.Context, args *args.Args) []generator.Target {
	var packageList []generator.Target

	groupGoNames := make(map[string]string)
	for _, inputDir := range args.InputDirs {
		p := context.Universe.Package(inputDir)

		var gv clientgentypes.GroupVersion

		parts := strings.Split(p.Path, "/")
		gv.Group = clientgentypes.Group(parts[len(parts)-2])
		gv.Version = clientgentypes.Version(parts[len(parts)-1])

		groupPackageName := gv.Group.NonEmpty()

		// If there's a comment of the form "// +groupName=somegroup" or
		// "// +groupName=somegroup.foo.bar.io", use the first field (somegroup) as the name of the
		// group when generating.
		if tags, err := gengo.ExtractFunctionStyleCommentTags("+", nil, p.Comments); err != nil {
			panic(err)
		} else if override := tags["groupName"]; override != nil {
			gv.Group = clientgentypes.Group(override[0].Value)
		}

		// If there's a comment of the form "// +groupGoName=SomeUniqueShortName", use that as
		// the Go group identifier in CamelCase. It defaults
		groupGoNames[groupPackageName] = namer.IC(strings.SplitN(gv.Group.NonEmpty(), ".", 2)[0])
		if tags, err := gengo.ExtractFunctionStyleCommentTags("+", nil, p.Comments); err != nil {
			panic(err)
		} else if override := tags["groupGoName"]; override != nil {
			groupGoNames[groupPackageName] = namer.IC(override[0].Value)
		}

		var typesWithInformers []*types.Type
		var duckTypes []*types.Type
		var reconcilerTypes []*types.Type

		for _, t := range p.Types {
			tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
			if tags.NeedsInformerInjection() {
				typesWithInformers = append(typesWithInformers, t)
			}
			if tags.NeedsDuckInjection() {
				duckTypes = append(duckTypes, t)
			}
			if tags.NeedsReconciler(t, args) {
				reconcilerTypes = append(reconcilerTypes, t)
			}
		}

		sources := []struct {
			types     []*types.Type
			generator genFunc
		}{
			{types: typesWithInformers, generator: informers.Targets},
			{types: duckTypes, generator: duck.Targets},
			{types: reconcilerTypes, generator: reconciler.Targets},
		}

		for _, source := range sources {
			if len(source.types) != 0 {
				orderer := namer.Orderer{Namer: namer.NewPrivateNamer(0)}
				source.types = orderer.OrderTypes(source.types)
				targets := source.generator(args, groupPackageName, gv, groupGoNames[groupPackageName], source.types)
				packageList = append(packageList, targets...)
			}
		}
	}

	// Generate the client and fake.
	packageList = append(packageList, client.Targets(args)...)

	// // Generate the informer factory and fake.
	packageList = append(packageList, factory.Targets(args)...)

	return packageList
}
