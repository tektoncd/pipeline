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

package factory

import (
	"path/filepath"

	"k8s.io/gengo/v2/generator"
	"k8s.io/gengo/v2/types"
	"knative.dev/pkg/codegen/cmd/injection-gen/args"
	"knative.dev/pkg/codegen/cmd/injection-gen/generators/client"
	"knative.dev/pkg/codegen/cmd/injection-gen/tags"
)

func Targets(args *args.Args) []generator.Target {
	packagePath := filepath.Join(args.GetOutputPackagePath(), "informers", "factory")
	packageDir := filepath.Join(args.GetOutputDir(), "informers", "factory")

	clientGen := client.New(args)
	clientFakeGen := client.NewFake(args)

	return []generator.Target{
		// Impl
		&generator.SimpleTarget{
			PkgName:       "factory",
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&factoryGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "factory.go",
					},
					outputPackage:                packagePath,
					cachingClientSetPackage:      clientGen.OutputPackagePath(),
					sharedInformerFactoryPackage: args.ExternalVersionsInformersPackage,
					imports:                      generator.NewImportTracker(),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		},
		// Fake
		&generator.SimpleTarget{
			PkgName:       "fake",
			PkgPath:       filepath.Join(packagePath, "fake"),
			PkgDir:        filepath.Join(packageDir, "fake"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&fakeFactoryGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "fake.go",
					},
					outputPackage:                filepath.Join(packagePath, "fake"),
					factoryInjectionPkg:          packagePath,
					fakeClientInjectionPkg:       clientFakeGen.OutputPackagePath(),
					sharedInformerFactoryPackage: args.ExternalVersionsInformersPackage,
					imports:                      generator.NewImportTracker(),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		},

		// FilterFactoryImpl
		&generator.SimpleTarget{
			PkgName:       "filteredFactory",
			PkgPath:       filepath.Join(packagePath, "filtered"),
			PkgDir:        filepath.Join(packageDir, "filtered"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&filteredFactoryGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "filtered_factory.go",
					},
					outputPackage:                filepath.Join(packagePath, "filtered"),
					cachingClientSetPackage:      clientGen.OutputPackagePath(),
					sharedInformerFactoryPackage: args.ExternalVersionsInformersPackage,
					imports:                      generator.NewImportTracker(),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		},
		// FakeFilterFactory
		&generator.SimpleTarget{
			PkgName:       "fakeFilteredFactory",
			PkgPath:       filepath.Join(packagePath, "filtered", "fake"),
			PkgDir:        filepath.Join(packageDir, "filtered", "fake"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&fakeFilteredFactoryGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "fake_filtered_factory.go",
					},
					outputPackage:                filepath.Join(packagePath, "filtered", "fake"),
					factoryInjectionPkg:          filepath.Join(packagePath, "filtered"),
					fakeClientInjectionPkg:       clientFakeGen.OutputPackagePath(),
					sharedInformerFactoryPackage: args.ExternalVersionsInformersPackage,
					imports:                      generator.NewImportTracker(),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		},
	}
}
