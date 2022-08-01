package v1beta1

import (
	"context"

	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
)

func (s Step) convertTo(ctx context.Context, sink *v1.Step) {
	sink.Name = s.Name
	sink.Image = s.Image
	sink.Command = s.Command
	sink.Args = s.Args
	sink.WorkingDir = s.WorkingDir
	sink.EnvFrom = s.EnvFrom
	sink.Env = s.Env
	sink.Resources = s.Resources
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
	sink.OnError = s.OnError
	sink.StdoutConfig = (*v1.StepOutputConfig)(s.StdoutConfig)
	sink.StderrConfig = (*v1.StepOutputConfig)(s.StderrConfig)

	// TODO(#4546): Handle deprecated fields
	// Ports, LivenessProbe, ReadinessProbe, StartupProbe, Lifecycle, TerminationMessagePath
	// TerminationMessagePolicy, Stdin, StdinOnce, TTY
}

func (s *Step) convertFrom(ctx context.Context, source v1.Step) {
	s.Name = source.Name
	s.Image = source.Image
	s.Command = source.Command
	s.Args = source.Args
	s.WorkingDir = source.WorkingDir
	s.EnvFrom = source.EnvFrom
	s.Env = source.Env
	s.Resources = source.Resources
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
	s.OnError = source.OnError
	s.StdoutConfig = (*StepOutputConfig)(source.StdoutConfig)
	s.StderrConfig = (*StepOutputConfig)(source.StderrConfig)
}

func (s StepTemplate) convertTo(ctx context.Context, sink *v1.StepTemplate) {
	sink.Image = s.Image
	sink.Command = s.Command
	sink.Args = s.Args
	sink.WorkingDir = s.WorkingDir
	sink.EnvFrom = s.EnvFrom
	sink.Env = s.Env
	sink.Resources = s.Resources
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
	s.Resources = source.Resources
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
	sink.Resources = s.Resources
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
	s.Resources = source.Resources
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
