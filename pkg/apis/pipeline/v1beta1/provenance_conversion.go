/*
Copyright 2022 The Tekton Authors
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

package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (p Provenance) convertTo(ctx context.Context, sink *v1.Provenance) {
	if p.RefSource != nil {
		new := v1.RefSource{}
		p.RefSource.convertTo(ctx, &new)
		sink.RefSource = &new
	}
	if p.FeatureFlags != nil {
		sink.FeatureFlags = p.FeatureFlags
	}
}

func (p *Provenance) convertFrom(ctx context.Context, source v1.Provenance) {
	if source.RefSource != nil {
		new := RefSource{}
		new.convertFrom(ctx, *source.RefSource)
		p.RefSource = &new
	}
	if source.FeatureFlags != nil {
		p.FeatureFlags = source.FeatureFlags
	}
}

func (cs RefSource) convertTo(ctx context.Context, sink *v1.RefSource) {
	sink.URI = cs.URI
	sink.Digest = cs.Digest
	sink.EntryPoint = cs.EntryPoint
}

func (cs *RefSource) convertFrom(ctx context.Context, source v1.RefSource) {
	cs.URI = source.URI
	cs.Digest = source.Digest
	cs.EntryPoint = source.EntryPoint
}
