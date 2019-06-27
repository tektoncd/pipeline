/*
Copyright 2019 The Knative Authors

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

// Package injection defines the mechanisms through which clients, informers
// and shared informer factories are injected into a shared controller binary
// implementation.
//
// There are two primary contexts where the usage of the injection package is
// interesting.  The first is in the context of implementations of
// `controller.Reconciler` being wrapped in a `*controller.Impl`:
//
//   import (
//     // Simply linking this triggers the injection of the informer, which links
//     // the factory triggering its injection, and which links the client,
//     // triggering its injection.  All you need to know is that it works :)
//     deployinformer "github.com/knative/pkg/injection/informers/kubeinformers/appsv1/deployment"
//     "github.com/knative/pkg/injection"
//   )
//
//   func NewController(ctx context.Context) *controller.Impl {
//     deploymentInformer := deployinformer.Get(ctx)
//     // Pass deploymentInformer.Lister() to Reconciler
//     ...
//     // Set up events on deploymentInformer.Informer()
//     ...
//   }
//
// Then in `package main` the entire controller process can be set up via:
//
//   package main
//
//   import (
//   	// The set of controllers this controller process runs.
//      // Linking these will register their transitive dependencies, after
//      // which the shared main can set up the rest.
//   	"github.com/knative/foo/pkg/reconciler/matt"
//   	"github.com/knative/foo/pkg/reconciler/scott"
//   	"github.com/knative/foo/pkg/reconciler/ville"
//   	"github.com/knative/foo/pkg/reconciler/dave"
//
//   	// This defines the shared main for injected controllers.
//   	"github.com/knative/pkg/injection/sharedmain"
//   )
//
//   func main() {
//   	sharedmain.Main("my-component",
//         // We pass in the list of controllers to construct, and that's it!
//         // If we forget to add this, go will complain about the unused import.
//         matt.NewController,
//         scott.NewController,
//         ville.NewController,
//         dave.NewController,
//      )
//   }
package injection
