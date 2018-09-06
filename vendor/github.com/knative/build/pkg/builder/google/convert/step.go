/*
Copyright 2018 The Knative Authors

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

package convert

import (
	corev1 "k8s.io/api/core/v1"

	"google.golang.org/api/cloudbuild/v1"

	"github.com/knative/build/pkg/builder/validation"
)

func ToContainerFromStep(og *cloudbuild.BuildStep) (*corev1.Container, error) {
	e, err := ToEnvFromAssociativeList(og.Env)
	if err != nil {
		return nil, err
	}
	vms, err := ToVolumeMountsFromVolumes(og.Volumes)
	if err != nil {
		return nil, err
	}
	command := []string{}
	if og.Entrypoint != "" {
		command = append(command, og.Entrypoint)
	}
	return &corev1.Container{
		Name:         og.Id,
		Image:        og.Name,
		WorkingDir:   og.Dir,
		Command:      command,
		Args:         og.Args,
		Env:          e,
		VolumeMounts: vms,
	}, nil
}

func ToStepFromContainer(og *corev1.Container) (*cloudbuild.BuildStep, error) {
	al, err := ToAssociativeListFromEnv(og.Env)
	if err != nil {
		return nil, err
	}
	vs, err := ToVolumesFromVolumeMounts(og.VolumeMounts)
	if err != nil {
		return nil, err
	}
	var ep string
	switch {
	case len(og.Command) == 1:
		ep = og.Command[0]
	case len(og.Command) > 1:
		// TODO(mattmoor): This is a restriction we should eliminate.  It also isn't clear that this
		// is 100% fidelity translation, since Dockerfile's scalar vs. list semantics are not what this
		// is doing (though the semantics of GCB and Kubernetes aren't necessarily 1:1 with that).
		return nil, validation.NewError("UnsupportedCommand", "the Google builder doesn't support multi-element commands, got: %v", og.Command)
	}
	return &cloudbuild.BuildStep{
		Id:         og.Name,
		Name:       og.Image,
		Dir:        og.WorkingDir,
		Entrypoint: ep,
		Args:       og.Args,
		Env:        al,
		Volumes:    vs,
	}, nil
}
