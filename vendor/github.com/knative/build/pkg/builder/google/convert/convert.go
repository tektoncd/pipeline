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
	"encoding/json"
	"errors"
	"fmt"
	"reflect"

	cloudbuild "google.golang.org/api/cloudbuild/v1"
	corev1 "k8s.io/api/core/v1"

	v1alpha1 "github.com/knative/build/pkg/apis/build/v1alpha1"
	"github.com/knative/build/pkg/builder/validation"
)

const (
	customSource = "custom-source"
)

var (
	emptyVolumeSource = corev1.VolumeSource{
		EmptyDir: &corev1.EmptyDirVolumeSource{},
	}
	socketType               = corev1.HostPathSocket
	daemonSocketVolumeSource = corev1.VolumeSource{
		HostPath: &corev1.HostPathVolumeSource{
			Path: "/var/run/docker.sock",
			Type: &socketType,
		},
	}
)

func remarshal(in, out interface{}) error {
	bts, err := json.Marshal(in)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(bts, &out); err != nil {
		return err
	}
	return nil
}

func FromCRD(u *v1alpha1.BuildSpec) (*cloudbuild.Build, error) {
	if u.ServiceAccountName != "" {
		return nil, fmt.Errorf("Unsupported: ServiceAccountName with Google builder, got: %v", u.ServiceAccountName)
	}
	bld := cloudbuild.Build{
		Steps: make([]*cloudbuild.BuildStep, 0, len(u.Steps)),
	}
	if u.Source != nil {
		switch {
		case u.Source.Git != nil:
			return nil, errors.New("Unsupported: Git source with Google builder")
		case u.Source.GCS != nil:
			ss, err := ToStorageSourceFromGCS(u.Source.GCS)
			if err != nil {
				return nil, err
			}
			bld.Source = &cloudbuild.Source{
				StorageSource: ss,
			}
		case u.Source.Custom != nil:
			step, err := ToStepFromContainer(u.Source.Custom)
			if err != nil {
				return nil, err
			}
			step.Id = customSource
			bld.Steps = append(bld.Steps, step)

		default:
			return nil, validation.NewError("UnsupportedSource", "unsupported Source, got %v", u.Source)
		}
	}
	// We only support roundtripping emptyDir volumes, but we support hostPath volumes
	// of the daemon socket as input, which GCB provides to all steps whether they need
	// it or not.
	socketVolume := ""
	for _, v := range u.Volumes {
		switch {
		case reflect.DeepEqual(v.VolumeSource, daemonSocketVolumeSource):
			// This is a lossy translation, we cannot roundtrip this without
			// upgrading every ToCRD to a privileged Build.
			socketVolume = v.Name
		case reflect.DeepEqual(v.VolumeSource, emptyVolumeSource):
		default:
			return nil, validation.NewError("UnsupportedVolume", "only certain volumes are supported on the Google builder, got %v", v.VolumeSource)
		}
	}
	for _, c := range u.Steps {
		step, err := ToStepFromContainer(&c)
		if err != nil {
			return nil, err
		}
		var vms []*cloudbuild.Volume
		for _, vm := range step.Volumes {
			if vm.Name == socketVolume {
				continue
			}
			vms = append(vms, vm)
		}
		step.Volumes = vms
		bld.Steps = append(bld.Steps, step)
	}
	return &bld, nil
}

func ToCRD(u *cloudbuild.Build) (*v1alpha1.BuildSpec, error) {
	bld := v1alpha1.BuildSpec{
		Steps: make([]corev1.Container, 0, len(u.Steps)),
	}
	steps := u.Steps
	switch {
	case u.Source != nil:
		switch {
		case u.Source.RepoSource != nil:
			return nil, errors.New("Unsupported: Git source with Google builder")
		case u.Source.StorageSource != nil:
			bld.Source = &v1alpha1.SourceSpec{
				GCS: ToGCSFromStorageSource(u.Source.StorageSource),
			}
		}
	case steps[0].Id == customSource:
		c, err := ToContainerFromStep(steps[0])
		if err != nil {
			return nil, err
		}
		c.Name = ""
		steps = steps[1:]
		bld.Source = &v1alpha1.SourceSpec{
			Custom: c,
		}
	case u.Source != nil:
		return nil, validation.NewError("UnsupportedSource", "unsupported Source, got: %v", u.Source)
	}
	volumeNames := make(map[string]bool)
	for _, step := range steps {
		c, err := ToContainerFromStep(step)
		if err != nil {
			return nil, err
		}
		for _, v := range c.VolumeMounts {
			volumeNames[v.Name] = true
		}
		bld.Steps = append(bld.Steps, *c)
	}

	// Create emptyDir volume entries, which is all GCB supports.
	for k, _ := range volumeNames {
		bld.Volumes = append(bld.Volumes, corev1.Volume{
			Name:         k,
			VolumeSource: emptyVolumeSource,
		})
	}

	return &bld, nil
}
