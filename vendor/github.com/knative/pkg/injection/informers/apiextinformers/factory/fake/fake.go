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

package fake

import (
	"context"

	informers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/injection"
	"github.com/knative/pkg/injection/clients/apiextclient/fake"
	"github.com/knative/pkg/injection/informers/apiextinformers/factory"
)

var Get = factory.Get

func init() {
	injection.Fake.RegisterInformerFactory(withInformerFactory)
}

func withInformerFactory(ctx context.Context) context.Context {
	kc := fake.Get(ctx)
	return context.WithValue(ctx, factory.Key{},
		informers.NewSharedInformerFactory(kc, controller.GetResyncPeriod(ctx)))
}
