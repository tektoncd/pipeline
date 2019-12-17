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

package config

import (
	"context"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/artifacts"
	"knative.dev/pkg/configmap"
)

type cfgKey struct{}

// +k8s:deepcopy-gen=false
type Config struct {
	ArtifactBucket *v1alpha1.ArtifactBucket
}

func FromContext(ctx context.Context) *Config {
	return ctx.Value(cfgKey{}).(*Config)
}

func ToContext(ctx context.Context, c *Config) context.Context {
	return context.WithValue(ctx, cfgKey{}, c)
}

// +k8s:deepcopy-gen=false
type Store struct {
	*configmap.UntypedStore

	images pipeline.Images
}

func NewStore(images pipeline.Images, logger configmap.Logger) *Store {
	return &Store{
		UntypedStore: configmap.NewUntypedStore(
			"pipelinerun",
			logger,
			configmap.Constructors{
				artifacts.GetBucketConfigName(): artifacts.NewArtifactBucketConfigFromConfigMap(images),
			},
		),
		images: images,
	}
}

func (s *Store) ToContext(ctx context.Context) context.Context {
	return ToContext(ctx, s.Load())
}

func (s *Store) Load() *Config {
	ep := s.UntypedLoad(artifacts.GetBucketConfigName())
	if ep == nil {
		return &Config{
			ArtifactBucket: &v1alpha1.ArtifactBucket{
				Location:    "",
				ShellImage:  s.images.ShellImage,
				GsutilImage: s.images.GsutilImage,
			},
		}
	}

	return &Config{
		ArtifactBucket: ep.(*v1alpha1.ArtifactBucket).DeepCopy(),
	}

}
