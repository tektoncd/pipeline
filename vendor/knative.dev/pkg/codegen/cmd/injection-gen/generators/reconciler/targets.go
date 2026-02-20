/*
Copyright 2024 The Knative Authors

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

package reconciler

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
	basePackage := args.GetOutputPackagePath()
	packagePath := filepath.Join(basePackage, "reconciler", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))
	packageDir := filepath.Join(args.GetOutputDir(), "reconciler", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))

	clientPackagePath := filepath.Join(basePackage, "client")

	vers := make([]generator.Target, 0, 4*len(typesToGenerate))

	for _, t := range typesToGenerate {
		extracted := extractCommentTags(t)
		reconcilerClasses, hasReconcilerClass := extractReconcilerClassesTag(extracted)
		nonNamespaced := isNonNamespaced(extracted)
		isKRShaped := isKRShaped(extracted)
		stubs := stubs(extracted)

		packagePath := filepath.Join(packagePath, strings.ToLower(t.Name.Name))
		packageDir := filepath.Join(packageDir, strings.ToLower(t.Name.Name))

		informerPackagePath := filepath.Join(basePackage, "informers", groupPkgName, strings.ToLower(gv.Version.NonEmpty()), strings.ToLower(t.Name.Name))
		listerPackagePath := filepath.Join(args.ListersPackage, groupPkgName, strings.ToLower(gv.Version.NonEmpty()))

		// Controller
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       strings.ToLower(t.Name.Name),
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&reconcilerControllerGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "controller.go",
					},
					typeToGenerate:      t,
					outputPackage:       packagePath,
					imports:             generator.NewImportTracker(),
					groupName:           gv.Group.String(),
					clientPkg:           clientPackagePath,
					informerPackagePath: informerPackagePath,
					schemePkg:           filepath.Join(args.VersionedClientSetPackage, "scheme"),
					reconcilerClasses:   reconcilerClasses,
					hasReconcilerClass:  hasReconcilerClass,
					hasStatus:           hasStatus(t),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsReconciler(t, args)
			},
		})

		if stubs {
			// Controller Stub
			vers = append(vers, &generator.SimpleTarget{
				PkgName:       strings.ToLower(t.Name.Name),
				PkgPath:       filepath.Join(packagePath, "stub"),
				PkgDir:        filepath.Join(packageDir, "stub"),
				HeaderComment: args.Boilerplate,
				GeneratorsFunc: func(c *generator.Context) []generator.Generator {
					// Impl
					return []generator.Generator{&reconcilerControllerStubGenerator{
						GoGenerator: generator.GoGenerator{
							OutputFilename: "controller.go",
						},
						typeToGenerate:      t,
						reconcilerPkg:       packagePath,
						outputPackage:       filepath.Join(packagePath, "stub"),
						imports:             generator.NewImportTracker(),
						informerPackagePath: informerPackagePath,
						reconcilerClasses:   reconcilerClasses,
						hasReconcilerClass:  hasReconcilerClass,
					}}
				},
				FilterFunc: func(c *generator.Context, t *types.Type) bool {
					tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
					return tags.NeedsReconciler(t, args)
				},
			})
		}

		// Reconciler
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       strings.ToLower(t.Name.Name),
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// Impl
				return []generator.Generator{&reconcilerReconcilerGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "reconciler.go",
					},
					typeToGenerate:     t,
					outputPackage:      packagePath,
					imports:            generator.NewImportTracker(),
					clientsetPkg:       args.VersionedClientSetPackage,
					listerName:         t.Name.Name + "Lister",
					listerPkg:          listerPackagePath,
					groupGoName:        groupGoName,
					groupVersion:       gv,
					reconcilerClasses:  reconcilerClasses,
					hasReconcilerClass: hasReconcilerClass,
					nonNamespaced:      nonNamespaced,
					isKRShaped:         isKRShaped,
					hasStatus:          hasStatus(t),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsReconciler(t, args)
			},
		})

		if stubs {
			// Reconciler Stub
			vers = append(vers, &generator.SimpleTarget{
				PkgName:       strings.ToLower(t.Name.Name),
				PkgPath:       filepath.Join(packagePath, "stub"),
				PkgDir:        filepath.Join(packageDir, "stub"),
				HeaderComment: args.Boilerplate,
				GeneratorsFunc: func(c *generator.Context) []generator.Generator {
					// Impl
					return []generator.Generator{&reconcilerReconcilerStubGenerator{
						GoGenerator: generator.GoGenerator{
							OutputFilename: "reconciler.go",
						},
						typeToGenerate: t,
						reconcilerPkg:  packagePath,
						outputPackage:  filepath.Join(packagePath, "stub"),
						imports:        generator.NewImportTracker(),
					}}
				},
				FilterFunc: func(c *generator.Context, t *types.Type) bool {
					tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
					return tags.NeedsReconciler(t, args)
				},
			})
		}

		// Reconciler State
		vers = append(vers, &generator.SimpleTarget{
			PkgName:       strings.ToLower(t.Name.Name),
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				// state
				return []generator.Generator{&reconcilerStateGenerator{
					GoGenerator: generator.GoGenerator{
						OutputFilename: "state.go",
					},
					typeToGenerate: t,
					outputPackage:  packagePath,
					imports:        generator.NewImportTracker(),
				}}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsReconciler(t, args)
			},
		})
	}
	return vers
}

func extractCommentTags(t *types.Type) tags.CommentTags {
	comments := append(append([]string{}, t.SecondClosestCommentLines...), t.CommentLines...)
	return tags.ExtractCommentTags("+", comments)
}

func extractReconcilerClassesTag(tags tags.CommentTags) ([]string, bool) {
	vals, ok := tags["genreconciler"]
	if !ok {
		return nil, false
	}
	classnames, has := vals["class"]
	if has && len(classnames) == 0 {
		return nil, false
	}
	return classnames, has
}

func isKRShaped(tags tags.CommentTags) bool {
	vals, has := tags["genreconciler"]
	if !has {
		return false
	}
	stringVals, has := vals["krshapedlogic"]
	if !has || len(vals) == 0 {
		return true // Default is true
	}
	return stringVals[0] != "false"
}

func isNonNamespaced(tags tags.CommentTags) bool {
	vals, has := tags["genclient"]
	if !has {
		return false
	}
	_, has = vals["nonNamespaced"]
	return has
}

func stubs(tags tags.CommentTags) bool {
	vals, has := tags["genreconciler"]
	if !has {
		return false
	}
	_, has = vals["stubs"]
	return has
}

func hasStatus(t *types.Type) bool {
	for _, member := range t.Members {
		if member.Name == "Status" {
			return true
		}
	}
	return false
}
