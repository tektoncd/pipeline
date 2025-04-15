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

package client

import (
	"path/filepath"

	"k8s.io/gengo/v2/generator"
	"k8s.io/gengo/v2/types"
	"knative.dev/pkg/codegen/cmd/injection-gen/args"
	"knative.dev/pkg/codegen/cmd/injection-gen/tags"
)

func Targets(args *args.Args) []generator.Target {
	packagePath := filepath.Join(args.GetOutputPackagePath(), "client")
	packageDir := filepath.Join(args.GetOutputDir(), "client")

	return []generator.Target{
		&generator.SimpleTarget{
			PkgName:       "client",
			PkgPath:       packagePath,
			PkgDir:        packageDir,
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				return []generator.Generator{New(args)}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		},
		&generator.SimpleTarget{
			PkgName:       "fake",
			PkgPath:       filepath.Join(packagePath, "fake"),
			PkgDir:        filepath.Join(packageDir, "fake"),
			HeaderComment: args.Boilerplate,
			GeneratorsFunc: func(c *generator.Context) []generator.Generator {
				return []generator.Generator{NewFake(args)}
			},
			FilterFunc: func(c *generator.Context, t *types.Type) bool {
				tags := tags.MustParseClientGenTags(append(t.SecondClosestCommentLines, t.CommentLines...))
				return tags.NeedsInformerInjection()
			},
		},
	}
}
