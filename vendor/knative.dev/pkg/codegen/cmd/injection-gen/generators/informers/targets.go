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

package informers

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
	factoryPackagePath := filepath.Join(args.GetOutputPackagePath(), "informers", "factory")
	filteredFactoryPackagePath := filepath.Join(args.GetOutputPackagePath(), "informers", "factory", "filtered")

	packagePath := filepath.Join(args.GetOutputPackagePath(), "informers", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))
	packageDir := filepath.Join(args.GetOutputDir(), "informers", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))

	vers := make([]generator.Target, 0, 2*len(typesToGenerate))

	for _, t := range typesToGenerate {
		packagePath := filepath.Join(packagePath, strings.ToLower(t.Name.Name))
		packageDir := filepath.Join(packageDir, strings.ToLower(t.Name.Name))
		typedInformerPackage := typedInformerPackage(groupPkgName, gv, args.ExternalVersionsInformersPackage)

		// Impl
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       strings.ToLower(t.Name.Name),
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&injectionGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: strings.ToLower(t.Name.Name) + ".go",
					},
					outputPackage:               packagePath,
					groupVersion:                gv,
					groupGoName:                 groupGoName,
					typeToGenerate:              t,
					imports:                     generator.NewImportTracker(),
					typedInformerPackage:        typedInformerPackage,
					groupInformerFactoryPackage: factoryPackagePath,
					disableInformerInit:         args.DisableInformerInit,
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
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
				return []generator.Generator{&fakeInformerGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "fake.go",
					},
					outputPackage:           filepath.Join(packagePath, "fake"),
					imports:                 generator.NewImportTracker(),
					typeToGenerate:          t,
					groupVersion:            gv,
					groupGoName:             groupGoName,
					informerInjectionPkg:    packagePath,
					fakeFactoryInjectionPkg: filepath.Join(factoryPackagePath, "fake"),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		})
		// FilteredInformer
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       "filtered",
			PkgPath:       filepath.Join(packagePath, "filtered"),
			PkgDir:        filepath.Join(packageDir, "filtered"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&filteredInjectionGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: strings.ToLower(t.Name.Name) + ".go",
					},
					outputPackage:               filepath.Join(packagePath, "filtered"),
					groupVersion:                gv,
					groupGoName:                 groupGoName,
					typeToGenerate:              t,
					imports:                     generator.NewImportTracker(),
					typedInformerPackage:        typedInformerPackage,
					groupInformerFactoryPackage: filteredFactoryPackagePath,
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		})

		// FakeFilteredInformer
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       "fake",
			PkgPath:       filepath.Join(packagePath, "filtered", "fake"),
			PkgDir:        filepath.Join(packageDir, "filtered", "fake"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&fakeFilteredInformerGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "fake.go",
					},
					outputPackage:           filepath.Join(packagePath, "filtered", "fake"),
					imports:                 generator.NewImportTracker(),
					typeToGenerate:          t,
					groupVersion:            gv,
					groupGoName:             groupGoName,
					informerInjectionPkg:    filepath.Join(packagePath, "filtered"),
					fakeFactoryInjectionPkg: filteredFactoryPackagePath,
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		})
	}
	return vers
}

func typedInformerPackage(groupPkgName string, gv clientgentypes.GroupVersion, externalVersionsInformersPackage string) string {
	return filepath.Join(externalVersionsInformersPackage, groupPkgName, gv.Version.String())
}
