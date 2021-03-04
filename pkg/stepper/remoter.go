/*
Copyright 2019 The Tekton Authors

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

package stepper

import (
	"context"
	"fmt"
	"github.com/google/go-containerregistry/pkg/authn/k8schain"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	gitclient "github.com/tektoncd/pipeline/pkg/git"
	"github.com/tektoncd/pipeline/pkg/remote"
	"github.com/tektoncd/pipeline/pkg/remote/git"
	"github.com/tektoncd/pipeline/pkg/remote/oci"
	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"
)

type RemoterOptions struct {
	KubeClient        kubernetes.Interface
	Namespace         string
	OCIServiceAccount string
	Logger            *zap.SugaredLogger
	GitOptions        gitclient.FetchSpec
}

func (o *RemoterOptions) CreateRemote(ctx context.Context, uses *v1beta1.Uses) (remote.Resolver, error) {
	if uses.Kind == v1beta1.UsesTypeOCI {
		bundle := uses.Path
		kc, err := k8schain.New(ctx, o.KubeClient, k8schain.Options{
			Namespace:          o.Namespace,
			ServiceAccountName: o.OCIServiceAccount,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get keychain: %w", err)
		}
		return oci.NewResolver(bundle, kc), nil
	}
	server := uses.Server
	if server == "" {
		server = "github.com"
	}
	return git.NewResolver(server, o.Logger, o.GitOptions), nil
}
