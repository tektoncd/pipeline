/*
Copyright 2018 The Knative Authors.

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

	"github.com/knative/pkg/configmap"
)

type cfgKey struct{}

// +k8s:deepcopy-gen=false
type Config struct {
	Entrypoint *Entrypoint
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
}

func NewStore(logger configmap.Logger) *Store {
	store := &Store{
		UntypedStore: configmap.NewUntypedStore(
			"taskrun",
			logger,
			configmap.Constructors{
				EntrypointConfigName: NewEntrypointConfigFromConfigMap,
			},
		),
	}

	return store
}

func (s *Store) ToContext(ctx context.Context) context.Context {
	return ToContext(ctx, s.Load())
}

func (s *Store) Load() *Config {
	ep := s.UntypedLoad(EntrypointConfigName)
	if ep == nil {
		return &Config{
			Entrypoint: &Entrypoint{
				Image: DefaultEntrypointImage,
			},
		}
	}

	return &Config{
		Entrypoint: ep.(*Entrypoint).DeepCopy(),
	}

}
