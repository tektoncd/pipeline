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

package v1alpha1

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

// GCSArtifactType defines a type of GCS resource.
type GCSArtifactType string

const (
	// GCSZipArchive indicates that the resource should be fetched and
	// extracted as a .zip file.
	//
	// Deprecated: Use GCSZipArchive instead.
	GCSArchive GCSArtifactType = "Archive"

	// GCSZipArchive indicates that the resource should be fetched and
	// extracted as a .zip file.
	GCSZipArchive GCSArtifactType = "ZipArchive"

	// GCSTarGzArchive indicates that the resource should be fetched and
	// extracted as a .tar.gz file.
	GCSTarGzArchive GCSArtifactType = "TarGzArchive"

	// GCSManifest indicates that resource should be fetched using a
	// manifest-based protocol which enables incremental source upload.
	GCSManifest GCSArtifactType = "Manifest"
)

var validArtifactTypes = []GCSArtifactType{
	GCSArchive,
	GCSManifest,
	GCSZipArchive,
	GCSTarGzArchive,
}

// BuildGCSResource describes a resource in the form of an archive,
// or a source manifest describing files to fetch.
// BuildGCSResource does incremental uploads for files in  directory.
type BuildGCSResource struct {
	Name         string
	Type         PipelineResourceType
	Location     string
	ArtifactType GCSArtifactType

	ShellImage           string `json:"-"`
	BuildGCSFetcherImage string `json:"-"`
}

// NewBuildGCSResource creates a new BuildGCS resource to pass to a Task.
func NewBuildGCSResource(images pipeline.Images, r *PipelineResource) (*BuildGCSResource, error) {
	if r.Spec.Type != PipelineResourceTypeStorage {
		return nil, fmt.Errorf("BuildGCSResource: Cannot create a BuildGCS resource from a %s Pipeline Resource", r.Spec.Type)
	}
	if r.Spec.SecretParams != nil {
		return nil, fmt.Errorf("BuildGCSResource: %s cannot support artifacts on private bucket", r.Name)
	}
	var location string
	var aType GCSArtifactType
	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "Location"):
			location = param.Value
		case strings.EqualFold(param.Name, "ArtifactType"):
			var err error
			aType, err = getArtifactType(param.Value)
			if err != nil {
				return nil, fmt.Errorf("BuildGCSResource %s : %w", r.Name, err)
			}
		}
	}
	if location == "" {
		return nil, fmt.Errorf("BuildGCSResource: Need Location to be specified in order to create BuildGCS resource %s", r.Name)
	}
	if aType == GCSArtifactType("") {
		return nil, fmt.Errorf("BuildGCSResource: Need ArtifactType to be specified to create BuildGCS resource %s", r.Name)
	}
	return &BuildGCSResource{
		Name:                 r.Name,
		Type:                 r.Spec.Type,
		Location:             location,
		ArtifactType:         aType,
		ShellImage:           images.ShellImage,
		BuildGCSFetcherImage: images.BuildGCSFetcherImage,
	}, nil
}

// GetName returns the name of the resource.
func (s BuildGCSResource) GetName() string { return s.Name }

// GetType returns the type of the resource, in this case "storage".
func (s BuildGCSResource) GetType() PipelineResourceType { return PipelineResourceTypeStorage }

// GetSecretParams returns nil because it takes no secret params.
func (s *BuildGCSResource) GetSecretParams() []SecretParam { return nil }

// Replacements returns the set of available replacements for this resource.
func (s *BuildGCSResource) Replacements() map[string]string {
	return map[string]string{
		"name":     s.Name,
		"type":     string(s.Type),
		"location": s.Location,
	}
}

// GetInputTaskModifier returns a TaskModifier that prepends a step to a Task to fetch the archive or manifest.
func (s *BuildGCSResource) GetInputTaskModifier(ts *TaskSpec, sourcePath string) (TaskModifier, error) {
	args := []string{"--type", string(s.ArtifactType), "--location", s.Location}
	// dest_dir is the destination directory for GCS files to be copies"
	if sourcePath != "" {
		args = append(args, "--dest_dir", sourcePath)
	}

	steps := []Step{
		CreateDirStep(s.ShellImage, s.Name, sourcePath),
		{Container: corev1.Container{
			Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("storage-fetch-%s", s.Name)),
			Command: []string{"/ko-app/gcs-fetcher"},
			Image:   s.BuildGCSFetcherImage,
			Args:    args,
		}}}

	volumes := getStorageVolumeSpec(s, *ts)

	return &InternalTaskModifier{
		StepsToPrepend: steps,
		Volumes:        volumes,
	}, nil
}

// GetOutputTaskModifier returns a No-op TaskModifier.
func (s *BuildGCSResource) GetOutputTaskModifier(ts *TaskSpec, sourcePath string) (TaskModifier, error) {
	return &InternalTaskModifier{}, nil
}

func getArtifactType(val string) (GCSArtifactType, error) {
	a := GCSArtifactType(val)
	for _, v := range validArtifactTypes {
		if a == v {
			return a, nil
		}
	}
	return "", fmt.Errorf("Invalid ArtifactType %s. Should be one of %s", val, validArtifactTypes)
}
