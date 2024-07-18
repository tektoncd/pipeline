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

package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (r Ref) convertTo(ctx context.Context, sink *v1.Ref) {
	sink.Name = r.Name
	new := v1.ResolverRef{}
	r.ResolverRef.convertTo(ctx, &new)
	sink.ResolverRef = new
}

func (r *Ref) convertFrom(ctx context.Context, source v1.Ref) {
	r.Name = source.Name
	new := ResolverRef{}
	new.convertFrom(ctx, source.ResolverRef)
	r.ResolverRef = new
}

func (s Step) convertTo(ctx context.Context, sink *v1.Step) {
	sink.Name = s.Name
	sink.Image = s.Image
	sink.Command = s.Command
	sink.Args = s.Args
	sink.WorkingDir = s.WorkingDir
	sink.EnvFrom = s.EnvFrom
	sink.Env = s.Env
	sink.ComputeResources = s.Resources
	sink.VolumeMounts = s.VolumeMounts
	sink.VolumeDevices = s.VolumeDevices
	sink.ImagePullPolicy = s.ImagePullPolicy
	sink.SecurityContext = s.SecurityContext
	sink.Script = s.Script
	sink.Timeout = s.Timeout

	sink.Workspaces = nil
	for _, w := range s.Workspaces {
		new := v1.WorkspaceUsage{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
	sink.OnError = (v1.OnErrorType)(s.OnError)
	sink.StdoutConfig = (*v1.StepOutputConfig)(s.StdoutConfig)
	sink.StderrConfig = (*v1.StepOutputConfig)(s.StderrConfig)
	if s.Ref != nil {
		sink.Ref = &v1.Ref{}
		s.Ref.convertTo(ctx, sink.Ref)
	}
	sink.Params = nil
	for _, p := range s.Params {
		new := v1.Param{}
		p.convertTo(ctx, &new)
		sink.Params = append(sink.Params, new)
	}
	sink.Results = s.Results
	for _, w := range s.When {
		new := v1.WhenExpression{}
		w.convertTo(ctx, &new)
		sink.When = append(sink.When, new)
	}
}

func (s *Step) convertFrom(ctx context.Context, source v1.Step) {
	s.Name = source.Name
	s.Image = source.Image
	s.Command = source.Command
	s.Args = source.Args
	s.WorkingDir = source.WorkingDir
	s.EnvFrom = source.EnvFrom
	s.Env = source.Env
	s.Resources = source.ComputeResources
	s.VolumeMounts = source.VolumeMounts
	s.VolumeDevices = source.VolumeDevices
	s.ImagePullPolicy = source.ImagePullPolicy
	s.SecurityContext = source.SecurityContext
	s.Script = source.Script
	s.Timeout = source.Timeout

	s.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspaceUsage{}
		new.convertFrom(ctx, w)
		s.Workspaces = append(s.Workspaces, new)
	}
	s.OnError = (OnErrorType)(source.OnError)
	s.StdoutConfig = (*StepOutputConfig)(source.StdoutConfig)
	s.StderrConfig = (*StepOutputConfig)(source.StderrConfig)
	if source.Ref != nil {
		newRef := Ref{}
		newRef.convertFrom(ctx, *source.Ref)
		s.Ref = &newRef
	}
	s.Params = nil
	for _, p := range source.Params {
		new := Param{}
		new.ConvertFrom(ctx, p)
		s.Params = append(s.Params, new)
	}
	s.Results = source.Results
	for _, w := range source.When {
		new := WhenExpression{}
		new.convertFrom(ctx, w)
		s.When = append(s.When, new)
	}
}

func (s StepTemplate) convertTo(ctx context.Context, sink *v1.StepTemplate) {
	sink.Image = s.Image
	sink.Command = s.Command
	sink.Args = s.Args
	sink.WorkingDir = s.WorkingDir
	sink.EnvFrom = s.EnvFrom
	sink.Env = s.Env
	sink.ComputeResources = s.Resources
	sink.VolumeMounts = s.VolumeMounts
	sink.VolumeDevices = s.VolumeDevices
	sink.ImagePullPolicy = s.ImagePullPolicy
	sink.SecurityContext = s.SecurityContext
	// TODO(#4546): Handle deprecated fields
	// Name, Ports, LivenessProbe, ReadinessProbe, StartupProbe, Lifecycle, TerminationMessagePath
	// TerminationMessagePolicy, Stdin, StdinOnce, TTY
}

func (s *StepTemplate) convertFrom(ctx context.Context, source *v1.StepTemplate) {
	s.Image = source.Image
	s.Command = source.Command
	s.Args = source.Args
	s.WorkingDir = source.WorkingDir
	s.EnvFrom = source.EnvFrom
	s.Env = source.Env
	s.Resources = source.ComputeResources
	s.VolumeMounts = source.VolumeMounts
	s.VolumeDevices = source.VolumeDevices
	s.ImagePullPolicy = source.ImagePullPolicy
	s.SecurityContext = source.SecurityContext
}

func (s Sidecar) convertTo(ctx context.Context, sink *v1.Sidecar) {
	sink.Name = s.Name
	sink.Image = s.Image
	sink.Command = s.Command
	sink.Args = s.Args
	sink.WorkingDir = s.WorkingDir
	sink.Ports = s.Ports
	sink.EnvFrom = s.EnvFrom
	sink.Env = s.Env
	sink.ComputeResources = s.Resources
	sink.VolumeMounts = s.VolumeMounts
	sink.VolumeDevices = s.VolumeDevices
	sink.LivenessProbe = s.LivenessProbe
	sink.ReadinessProbe = s.ReadinessProbe
	sink.StartupProbe = s.StartupProbe
	sink.Lifecycle = s.Lifecycle
	sink.TerminationMessagePath = s.TerminationMessagePath
	sink.TerminationMessagePolicy = s.TerminationMessagePolicy
	sink.ImagePullPolicy = s.ImagePullPolicy
	sink.SecurityContext = s.SecurityContext
	sink.Stdin = s.Stdin
	sink.StdinOnce = s.StdinOnce
	sink.TTY = s.TTY
	sink.Script = s.Script
	sink.Workspaces = nil
	for _, w := range s.Workspaces {
		new := v1.WorkspaceUsage{}
		w.convertTo(ctx, &new)
		sink.Workspaces = append(sink.Workspaces, new)
	}
}

func (s *Sidecar) convertFrom(ctx context.Context, source v1.Sidecar) {
	s.Name = source.Name
	s.Image = source.Image
	s.Command = source.Command
	s.Args = source.Args
	s.WorkingDir = source.WorkingDir
	s.Ports = source.Ports
	s.EnvFrom = source.EnvFrom
	s.Env = source.Env
	s.Resources = source.ComputeResources
	s.VolumeMounts = source.VolumeMounts
	s.VolumeDevices = source.VolumeDevices
	s.LivenessProbe = source.LivenessProbe
	s.ReadinessProbe = source.ReadinessProbe
	s.StartupProbe = source.StartupProbe
	s.Lifecycle = source.Lifecycle
	s.TerminationMessagePath = source.TerminationMessagePath
	s.TerminationMessagePolicy = source.TerminationMessagePolicy
	s.ImagePullPolicy = source.ImagePullPolicy
	s.SecurityContext = source.SecurityContext
	s.Stdin = source.Stdin
	s.StdinOnce = source.StdinOnce
	s.TTY = source.TTY
	s.Script = source.Script
	s.Workspaces = nil
	for _, w := range source.Workspaces {
		new := WorkspaceUsage{}
		new.convertFrom(ctx, w)
		s.Workspaces = append(s.Workspaces, new)
	}
}
