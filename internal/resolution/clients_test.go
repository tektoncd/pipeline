/*
Copyright 2021 The Tekton Authors

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

package resolution

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	ttesting "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test"
)

// getClients is a test helper to construct the fakes needed for
// testing ref resolution.
func getClients(t *testing.T, conf *config.Config, d test.Data) (test.Assets, func()) {
	ctx, _ := ttesting.SetupFakeContext(t)
	ctx, cancel := context.WithCancel(ctx)
	if conf != nil {
		ctx = config.ToContext(ctx, conf)
	}

	clients, informers := test.SeedTestData(t, ctx, d)

	return test.Assets{
		Ctx:       ctx,
		Clients:   clients,
		Informers: informers,
	}, cancel
}
