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

package factory

import (
	"context"

	"k8s.io/client-go/informers"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/injection"
	"github.com/knative/pkg/injection/clients/kubeclient"
	"github.com/knative/pkg/logging"
)

func init() {
	injection.Default.RegisterInformerFactory(withInformerFactory)
}

// Key is used as the key for associating information
// with a context.Context.
type Key struct{}

func withInformerFactory(ctx context.Context) context.Context {
	kc := kubeclient.Get(ctx)
	return context.WithValue(ctx, Key{},
		informers.NewSharedInformerFactory(kc, controller.GetResyncPeriod(ctx)))
}

// Get extracts the Kubernetes InformerFactory from the context.
func Get(ctx context.Context) informers.SharedInformerFactory {
	untyped := ctx.Value(Key{})
	if untyped == nil {
		logging.FromContext(ctx).Panicf(
			"Unable to fetch %T from context.", (informers.SharedInformerFactory)(nil))
	}
	return untyped.(informers.SharedInformerFactory)
}
