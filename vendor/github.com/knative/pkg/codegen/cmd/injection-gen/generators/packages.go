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
	"fmt"
	"path"
	"path/filepath"
	"strings"

	"k8s.io/code-generator/cmd/client-gen/generators/util"
	clientgentypes "k8s.io/code-generator/cmd/client-gen/types"
	"k8s.io/gengo/args"
	"k8s.io/gengo/generator"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"k8s.io/klog"

	informergenargs "github.com/knative/pkg/codegen/cmd/injection-gen/args"
)

// Packages makes the client package definition.
func Packages(context *generator.Context, arguments *args.GeneratorArgs) generator.Packages {
	boilerplate, err := arguments.LoadGoBoilerplate()
	if err != nil {
		klog.Fatalf("Failed loading boilerplate: %v", err)
	}

	customArgs, ok := arguments.CustomArgs.(*informergenargs.CustomArgs)
	if !ok {
		klog.Fatalf("Wrong CustomArgs type: %T", arguments.CustomArgs)
	}

	versionPackagePath := filepath.Join(arguments.OutputPackagePath)

	var packageList generator.Packages
	typesForGroupVersion := make(map[clientgentypes.GroupVersion][]*types.Type)

	groupVersions := make(map[string]clientgentypes.GroupVersions)
	groupGoNames := make(map[string]string)
	for _, inputDir := range arguments.InputDirs {
		p := context.Universe.Package(vendorless(inputDir))

		objectMeta, _, err := objectMetaForPackage(p) // TODO: ignoring internal.
		if err != nil {
			klog.Fatal(err)
		}
		if objectMeta == nil {
			// no types in this package had genclient
			continue
		}

		var gv clientgentypes.GroupVersion
		var targetGroupVersions map[string]clientgentypes.GroupVersions

		parts := strings.Split(p.Path, "/")
		gv.Group = clientgentypes.Group(parts[len(parts)-2])
		gv.Version = clientgentypes.Version(parts[len(parts)-1])
		targetGroupVersions = groupVersions

		groupPackageName := gv.Group.NonEmpty()
		gvPackage := path.Clean(p.Path)

		// If there's a comment of the form "// +groupName=somegroup" or
		// "// +groupName=somegroup.foo.bar.io", use the first field (somegroup) as the name of the
		// group when generating.
		if override := types.ExtractCommentTags("+", p.Comments)["groupName"]; override != nil {
			gv.Group = clientgentypes.Group(override[0])
		}

		// If there's a comment of the form "// +groupGoName=SomeUniqueShortName", use that as
		// the Go group identifier in CamelCase. It defaults
		groupGoNames[groupPackageName] = namer.IC(strings.Split(gv.Group.NonEmpty(), ".")[0])
		if override := types.ExtractCommentTags("+", p.Comments)["groupGoName"]; override != nil {
			groupGoNames[groupPackageName] = namer.IC(override[0])
		}

		var typesToGenerate []*types.Type
		for _, t := range p.Types {
			tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
			if !tags.GenerateClient || tags.NoVerbs || !tags.HasVerb("list") || !tags.HasVerb("watch") {
				continue
			}

			typesToGenerate = append(typesToGenerate, t)

			if _, ok := typesForGroupVersion[gv]; !ok {
				typesForGroupVersion[gv] = []*types.Type{}
			}
			typesForGroupVersion[gv] = append(typesForGroupVersion[gv], t)
		}
		if len(typesToGenerate) == 0 {
			continue
		}

		groupVersionsEntry, ok := targetGroupVersions[groupPackageName]
		if !ok {
			groupVersionsEntry = clientgentypes.GroupVersions{
				PackageName: groupPackageName,
				Group:       gv.Group,
			}
		}
		groupVersionsEntry.Versions = append(groupVersionsEntry.Versions, clientgentypes.PackageVersion{Version: gv.Version, Package: gvPackage})
		targetGroupVersions[groupPackageName] = groupVersionsEntry

		orderer := namer.Orderer{Namer: namer.NewPrivateNamer(0)}
		typesToGenerate = orderer.OrderTypes(typesToGenerate)

		// Generate the client and fake.
		packageList = append(packageList, versionClientsPackages(versionPackagePath, groupPackageName, gv, groupGoNames[groupPackageName], boilerplate, typesToGenerate, customArgs)...)

		// Generate the informer factory and fake.
		packageList = append(packageList, versionFactoryPackages(versionPackagePath, groupPackageName, gv, groupGoNames[groupPackageName], boilerplate, typesToGenerate, customArgs)...)

		// Generate the informer and fake, for each type.
		packageList = append(packageList, versionInformerPackages(versionPackagePath, groupPackageName, gv, groupGoNames[groupPackageName], boilerplate, typesToGenerate, customArgs)...)
	}

	return packageList
}

// objectMetaForPackage returns the type of ObjectMeta used by package p.
func objectMetaForPackage(p *types.Package) (*types.Type, bool, error) {
	generatingForPackage := false
	for _, t := range p.Types {
		if !util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...)).GenerateClient {
			continue
		}
		generatingForPackage = true
		for _, member := range t.Members {
			if member.Name == "ObjectMeta" {
				return member.Type, isInternal(member), nil
			}
		}
	}
	if generatingForPackage {
		return nil, false, fmt.Errorf("unable to find ObjectMeta for any types in package %s", p.Path)
	}
	return nil, false, nil
}

// isInternal returns true if the tags for a member do not contain a json tag
func isInternal(m types.Member) bool {
	return !strings.Contains(m.Tags, "json")
}

func vendorless(p string) string {
	if pos := strings.LastIndex(p, "/vendor/"); pos != -1 {
		return p[pos+len("/vendor/"):]
	}
	return p
}

func typedInformerPackage(groupPkgName string, gv clientgentypes.GroupVersion, externalVersionsInformersPackage string) string {
	return filepath.Join(externalVersionsInformersPackage, groupPkgName, gv.Version.String())
}

func versionClientsPackages(basePackage string, groupPkgName string, gv clientgentypes.GroupVersion, groupGoName string, boilerplate []byte, typesToGenerate []*types.Type, customArgs *informergenargs.CustomArgs) []generator.Package {
	packagePath := filepath.Join(basePackage, "client")

	vers := make([]generator.Package, 0, 2)

	// Impl
	vers = append(vers, &generator.DefaultPackage{
		PackageName: "client",
		PackagePath: packagePath,
		HeaderText:  boilerplate,
		GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {
			// Impl
			generators = append(generators, &clientGenerator{
				DefaultGen: generator.DefaultGen{
					OptionalName: "client",
				},
				outputPackage:    packagePath,
				imports:          generator.NewImportTracker(),
				clientSetPackage: customArgs.VersionedClientSetPackage,
			})

			return generators
		},
		FilterFunc: func(c *generator.Context, t *types.Type) bool {
			tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
			return tags.GenerateClient && tags.HasVerb("list") && tags.HasVerb("watch")
		},
	})

	// Fake
	vers = append(vers, &generator.DefaultPackage{
		PackageName: "fake",
		PackagePath: packagePath + "/fake",
		HeaderText:  boilerplate,
		GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {

			// Impl
			generators = append(generators, &fakeClientGenerator{
				DefaultGen: generator.DefaultGen{
					OptionalName: "fake",
				},
				outputPackage:      packagePath + "/fake",
				imports:            generator.NewImportTracker(),
				fakeClientPkg:      customArgs.VersionedClientSetPackage + "/fake",
				clientInjectionPkg: packagePath,
			})

			return generators
		},
		FilterFunc: func(c *generator.Context, t *types.Type) bool {
			tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
			return tags.GenerateClient && tags.HasVerb("list") && tags.HasVerb("watch")
		},
	})

	return vers
}

func versionFactoryPackages(basePackage string, groupPkgName string, gv clientgentypes.GroupVersion, groupGoName string, boilerplate []byte, typesToGenerate []*types.Type, customArgs *informergenargs.CustomArgs) []generator.Package {
	packagePath := filepath.Join(basePackage, "informers", groupPkgName, "factory")

	vers := make([]generator.Package, 0, 2)

	// Impl
	vers = append(vers, &generator.DefaultPackage{
		PackageName: strings.ToLower(groupPkgName + "factory"),
		PackagePath: packagePath,
		HeaderText:  boilerplate,
		GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {
			// Impl
			generators = append(generators, &factoryGenerator{
				DefaultGen: generator.DefaultGen{
					OptionalName: groupPkgName + "factory",
				},
				outputPackage:                packagePath,
				cachingClientSetPackage:      fmt.Sprintf("%s/client", basePackage),
				sharedInformerFactoryPackage: customArgs.ExternalVersionsInformersPackage,
				imports:                      generator.NewImportTracker(),
			})

			return generators
		},
		FilterFunc: func(c *generator.Context, t *types.Type) bool {
			tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
			return tags.GenerateClient && tags.HasVerb("list") && tags.HasVerb("watch")
		},
	})

	// Fake
	vers = append(vers, &generator.DefaultPackage{
		PackageName: "fake",
		PackagePath: packagePath + "/fake",
		HeaderText:  boilerplate,
		GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {

			// Impl
			generators = append(generators, &fakeFactoryGenerator{
				DefaultGen: generator.DefaultGen{
					OptionalName: "fake",
				},
				outputPackage:                packagePath + "/fake",
				factoryInjectionPkg:          packagePath,
				fakeClientInjectionPkg:       fmt.Sprintf("%s/client/fake", basePackage),
				sharedInformerFactoryPackage: customArgs.ExternalVersionsInformersPackage,
				imports:                      generator.NewImportTracker(),
			})

			return generators
		},
		FilterFunc: func(c *generator.Context, t *types.Type) bool {
			tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
			return tags.GenerateClient && tags.HasVerb("list") && tags.HasVerb("watch")
		},
	})

	return vers
}

func versionInformerPackages(basePackage string, groupPkgName string, gv clientgentypes.GroupVersion, groupGoName string, boilerplate []byte, typesToGenerate []*types.Type, customArgs *informergenargs.CustomArgs) []generator.Package {
	factoryPackagePath := filepath.Join(basePackage, "informers", groupPkgName, "factory")
	packagePath := filepath.Join(basePackage, "informers", groupPkgName, strings.ToLower(gv.Version.NonEmpty()))

	vers := make([]generator.Package, 0, len(typesToGenerate))

	for _, t := range typesToGenerate {
		// Fix for golang iterator bug.
		t := t

		packagePath := packagePath + "/" + strings.ToLower(t.Name.Name)
		typedInformerPackage := typedInformerPackage(groupPkgName, gv, customArgs.ExternalVersionsInformersPackage)

		// Impl
		vers = append(vers, &generator.DefaultPackage{
			PackageName: strings.ToLower(t.Name.Name),
			PackagePath: packagePath,
			HeaderText:  boilerplate,
			GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {
				// Impl
				generators = append(generators, &injectionGenerator{
					DefaultGen: generator.DefaultGen{
						OptionalName: strings.ToLower(t.Name.Name),
					},
					outputPackage:               packagePath,
					groupVersion:                gv,
					groupGoName:                 groupGoName,
					typeToGenerate:              t,
					imports:                     generator.NewImportTracker(),
					typedInformerPackage:        typedInformerPackage,
					groupInformerFactoryPackage: factoryPackagePath,
				})

				return generators
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.GenerateClient && tags.HasVerb("list") && tags.HasVerb("watch")
			},
		})

		// Fake
		vers = append(vers, &generator.DefaultPackage{
			PackageName: "fake",
			PackagePath: packagePath + "/fake",
			HeaderText:  boilerplate,
			GeneratorFunc: func(c *generator.Context) (generators []generator.Generator) {
				// Impl
				generators = append(generators, &fakeInformerGenerator{
					DefaultGen: generator.DefaultGen{
						OptionalName: "fake",
					},
					outputPackage:           packagePath + "/fake",
					imports:                 generator.NewImportTracker(),
					typeToGenerate:          t,
					groupVersion:            gv,
					groupGoName:             groupGoName,
					informerInjectionPkg:    packagePath,
					fakeFactoryInjectionPkg: factoryPackagePath + "/fake",
				})

				return generators
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := util.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.GenerateClient && tags.HasVerb("list") && tags.HasVerb("watch")
			},
		})
	}
	return vers
}
