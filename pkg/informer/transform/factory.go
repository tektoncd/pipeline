/*
Copyright 2026 The Tekton Authors

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

package transform

import (
	"context"

	externalversions "github.com/tektoncd/pipeline/pkg/client/informers/externalversions"
	client "github.com/tektoncd/pipeline/pkg/client/injection/client"
	factory "github.com/tektoncd/pipeline/pkg/client/injection/informers/factory"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
)

func init() {
	injection.Default.RegisterInformerFactory(withTransformFactory)
}

// withTransformFactory creates a new informer factory with the cache transform
// applied. This factory replaces the default generated factory by using the
// same context key.
//
// This approach ensures that all Tekton informers use the transform to reduce
// memory usage without modifying the generated code.
func withTransformFactory(ctx context.Context) context.Context {
	c := client.Get(ctx)
	opts := make([]externalversions.SharedInformerOption, 0, 2)

	if injection.HasNamespaceScope(ctx) {
		opts = append(opts, externalversions.WithNamespace(injection.GetNamespaceScope(ctx)))
	}

	// Add the cache transform to strip unnecessary fields from cached objects
	opts = append(opts, externalversions.WithTransform(TransformForCache))

	// Use the same Key{} as the generated factory to override it
	return context.WithValue(ctx, factory.Key{},
		externalversions.NewSharedInformerFactoryWithOptions(c, controller.GetResyncPeriod(ctx), opts...))
}
