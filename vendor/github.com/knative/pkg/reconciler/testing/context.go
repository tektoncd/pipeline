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

package testing

import (
	"context"
	"testing"

	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"

	"github.com/knative/pkg/controller"
	"github.com/knative/pkg/injection"
	logtesting "github.com/knative/pkg/logging/testing"
)

func SetupFakeContext(t *testing.T) (context.Context, []controller.Informer) {
	ctx := logtesting.TestContextWithLogger(t)
	ctx = controller.WithEventRecorder(ctx, record.NewFakeRecorder(1000))
	return injection.Fake.SetupInformers(ctx, &rest.Config{})
}
