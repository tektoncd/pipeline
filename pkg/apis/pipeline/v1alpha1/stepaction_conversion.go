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

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"knative.dev/pkg/apis"
)

var _ apis.Convertible = (*StepAction)(nil)

// ConvertTo implements apis.Convertible
func (s *StepAction) ConvertTo(ctx context.Context, to apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch sink := to.(type) {
	case *v1beta1.StepAction:
		sink.ObjectMeta = s.ObjectMeta
		return s.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo implements apis.Convertible
func (ss *StepActionSpec) ConvertTo(ctx context.Context, sink *v1beta1.StepActionSpec) error {
	sink.Description = ss.Description
	sink.Image = ss.Image
	sink.Command = ss.Command
	sink.Args = ss.Args
	sink.Env = ss.Env
	sink.Script = ss.Script
	sink.WorkingDir = ss.WorkingDir
	sink.Params = ss.Params
	sink.Results = ss.Results
	sink.SecurityContext = ss.SecurityContext
	sink.VolumeMounts = ss.VolumeMounts

	return nil
}

// ConvertFrom implements apis.Convertible
func (s *StepAction) ConvertFrom(ctx context.Context, from apis.Convertible) error {
	if apis.IsInDelete(ctx) {
		return nil
	}
	switch source := from.(type) {
	case *v1beta1.StepAction:
		s.ObjectMeta = source.ObjectMeta
		return s.Spec.ConvertFrom(ctx, &source.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}

// ConvertFrom implements apis.Convertible
func (ss *StepActionSpec) ConvertFrom(ctx context.Context, source *v1beta1.StepActionSpec) error {
	ss.Description = source.Description
	ss.Image = source.Image
	ss.Command = source.Command
	ss.Args = source.Args
	ss.Env = source.Env
	ss.Script = source.Script
	ss.WorkingDir = source.WorkingDir

	ss.Params = source.Params
	ss.Results = source.Results
	ss.SecurityContext = source.SecurityContext
	ss.VolumeMounts = source.VolumeMounts

	return nil
}
