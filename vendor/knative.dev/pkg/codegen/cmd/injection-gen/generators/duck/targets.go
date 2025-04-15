/*
Copyright 2025 The Knative Authors

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

package duck

import (
	"path/filepath"
	"strings"

	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/v2/generator"
	"k8s.io/gengo/v2/types"
	"knative.dev/pkg/codegen/cmd/injection-gen/args"
	"knative.dev/pkg/codegen/cmd/injection-gen/tags"
)

func Targets(
	args *args.Args,
	groupPkgName string,
	gv clientgentypes.GroupVersion,
	groupGoName string,
	typesToGenerate []*types.Type,
) []generator.Target {
	packagePath := filepath.Join(args.GetOutputPackagePath(), "ducks", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))
	packageDir := filepath.Join(args.GetOutputDir(), "ducks", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))

	vers := make([]generator.Target, 0, 2*len(typesToGenerate))

	for _, t := range typesToGenerate {
		packagePath := filepath.Join(packagePath, strings.ToLower(t.Name.Name))
		packageDir := filepath.Join(packageDir, strings.ToLower(t.Name.Name))

		// Impl
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       strings.ToLower(t.Name.Name),
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&duckGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: strings.ToLower(t.Name.Name) + ".go",
					},
					outputPackage:  packagePath,
					groupVersion:   gv,
					groupGoName:    groupGoName,
					typeToGenerate: t,
					imports:        generator.NewImportTracker(),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsDuckInjection()
			},
		})

		// Fake
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       "fake",
			PkgPath:       filepath.Join(packagePath, "fake"),
			PkgDir:        filepath.Join(packageDir, "fake"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&fakeDuckGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "fake.go",
					},
					outputPackage:    filepath.Join(packagePath, "fake"),
					imports:          generator.NewImportTracker(),
					typeToGenerate:   t,
					groupVersion:     gv,
					groupGoName:      groupGoName,
					duckInjectionPkg: packagePath,
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsDuckInjection()
			},
		})
	}
	return vers
}
