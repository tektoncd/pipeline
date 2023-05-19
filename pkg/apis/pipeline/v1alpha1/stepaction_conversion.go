/*
Copyright 2023 The Tekton Authors
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

package v1alpha1

import (
	"context"
	"fmt"

	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*StepAction)(nil)

// ConvertTo implements apis.Convertible
func (s *StepAction) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *StepAction:
		sink.ObjectMeta = s.ObjectMeta
		return s.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ss *StepActionSpec) ConvertTo(ctx context.Context, sink *StepActionSpec) error {
	sink.Name = ss.Name
	sink.Image = ss.Image
	sink.Command = ss.Command
	sink.Args = ss.Args
	sink.WorkingDir = ss.WorkingDir
	sink.EnvFrom = ss.EnvFrom
	sink.Env = ss.Env
	sink.Script = ss.Script
	sink.Params = ss.Params
	sink.Results = ss.Results
	return nil
}

// ConvertFrom implements apis.Convertible
func (s *StepAction) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *StepAction:
		s.ObjectMeta = source.ObjectMeta
		return s.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", s)
	}
}

// ConvertFrom implements apis.Convertible
func (ss *StepActionSpec) ConvertFrom(ctx context.Context, source *StepActionSpec) error {
	ss.Name = source.Name
	ss.Image = source.Image
	ss.Command = source.Command
	ss.Args = source.Args
	ss.WorkingDir = source.WorkingDir
	ss.EnvFrom = source.EnvFrom
	ss.Env = source.Env
	ss.Script = source.Script
	ss.Params = source.Params
	ss.Results = source.Results
	return nil
}
