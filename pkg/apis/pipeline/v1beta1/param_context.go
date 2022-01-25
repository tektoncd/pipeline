// Copyright 2021 The Tekton Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1beta1

import (
	"context"
	"fmt"
)

// paramCtxKey is the unique type for referencing param information from
// a context.Context. See [context.Context.Value](https://pkg.go.dev/context#Context)
// for more details.
//
// +k8s:openapi-gen=false
type paramCtxKeyType struct{}

var (
	paramCtxKey = paramCtxKeyType{}
)

// paramCtxVal is the data type stored in the param context.
// This maps param names -> ParamSpec.
type paramCtxVal map[string]ParamSpec

// addContextParams adds the given Params to the param context. This only
// preserves the fields included in ParamSpec - Name and Type.
func addContextParams(ctx context.Context, in []Param) context.Context {
	if in == nil {
		return ctx
	}

	out := paramCtxVal{}
	// Copy map to ensure that contexts are unique.
	v := ctx.Value(paramCtxKey)
	if v != nil {
		for n, cps := range v.(paramCtxVal) {
			out[n] = cps
		}
	}
	for _, p := range in {
		// The user may have omitted type data. Fill this in in to normalize data.
		if v := p.Value; v.Type == "" {
			if len(v.ArrayVal) > 0 {
				p.Value.Type = ParamTypeArray
			}
			if v.StringVal != "" {
				p.Value.Type = ParamTypeString
			}
		}
		out[p.Name] = ParamSpec{
			Name: p.Name,
			Type: p.Value.Type,
		}
	}
	return context.WithValue(ctx, paramCtxKey, out)
}

// addContextParamSpec adds the given ParamSpecs to the param context.
func addContextParamSpec(ctx context.Context, in []ParamSpec) context.Context {
	if in == nil {
		return ctx
	}

	out := paramCtxVal{}
	// Copy map to ensure that contexts are unique.
	v := ctx.Value(paramCtxKey)
	if v != nil {
		for n, ps := range v.(paramCtxVal) {
			out[n] = ps
		}
	}
	for _, p := range in {
		cps := ParamSpec{
			Name:        p.Name,
			Type:        p.Type,
			Description: p.Description,
			Default:     p.Default,
		}
		out[p.Name] = cps
	}
	return context.WithValue(ctx, paramCtxKey, out)
}

// getContextParams returns the current context parameters overlayed with a
// given set of params. Overrides should generally be the current layer you
// are trying to evaluate. Any context params not in the overrides will default
// to a generic pass-through param of the given type (i.e. $(params.name) or
// $(params.name[*])).
func getContextParams(ctx context.Context, overlays ...Param) []Param {
	pv := paramCtxVal{}
	v := ctx.Value(paramCtxKey)
	if v == nil && len(overlays) == 0 {
		return nil
	}
	if v != nil {
		pv = v.(paramCtxVal)
	}
	out := make([]Param, 0, len(pv))

	// Overlays take precedence over any context params. Keep track of
	// these and automatically add them to the output.
	overrideSet := make(map[string]Param, len(overlays))
	for _, p := range overlays {
		overrideSet[p.Name] = p
		out = append(out, p)
	}

	// Include the rest of the context params.
	for _, ps := range pv {
		// Don't do anything for any overlay params - these are already
		// included.
		if _, ok := overrideSet[ps.Name]; ok {
			continue
		}

		// If there is no overlay, pass through the param to the next level.
		// e.g. for strings $(params.name), for arrays $(params.name[*]).
		p := Param{
			Name: ps.Name,
		}
		if ps.Type == ParamTypeString {
			p.Value = ArrayOrString{
				Type:      ParamTypeString,
				StringVal: fmt.Sprintf("$(params.%s)", ps.Name),
			}
		} else {
			p.Value = ArrayOrString{
				Type:     ParamTypeArray,
				ArrayVal: []string{fmt.Sprintf("$(params.%s[*])", ps.Name)},
			}
		}
		out = append(out, p)
	}

	return out
}

// getContextParamSpecs returns the current context ParamSpecs.
func getContextParamSpecs(ctx context.Context) []ParamSpec {
	v := ctx.Value(paramCtxKey)
	if v == nil {
		return nil
	}

	pv := v.(paramCtxVal)
	out := make([]ParamSpec, 0, len(pv))
	for _, ps := range pv {
		out = append(out, ParamSpec{
			Name:        ps.Name,
			Type:        ps.Type,
			Description: ps.Description,
			Default:     ps.Default,
		})
	}
	return out
}
