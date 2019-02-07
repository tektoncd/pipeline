/*
Copyright 2019 The Knative Authors.

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
	"flag"
	"fmt"
	"path/filepath"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

var (
	buildGCSFetcherImage = flag.String("gcs-fetcher-image", "gcr.io/cloud-builders/gcs-fetcher:latest",
		"The container image containing our GCS fetcher binary.")
	buildGCSUploaderImage = flag.String("gcs-uploader-image", "gcr.io/cloud-builders/gcs-uploader:latest",
		"The container image containing our GCS uploader binary.")
)

// GCSArtifactType defines a type of GCS resource.
type GCSArtifactType string

const (
	// GCSArchive indicates that resource should be fetched from a typical archive file.
	GCSArchive GCSArtifactType = "Archive"

	// GCSManifest indicates that resource should be fetched using a
	// manifest-based protocol which enables incremental source upload.
	GCSManifest GCSArtifactType = "Manifest"
)

// BuildGCSResource describes a resource in the form of an archive,
// or a source manifest describing files to fetch.
type BuildGCSResource struct {
	Name           string
	Type           PipelineResourceType
	Location       string
	DestinationDir string
	ArtifactType   GCSArtifactType
	//Secret holds a struct to indicate a field name and corresponding secret name to populate it
	Secrets []SecretParam `json:"secrets"`
}

// NewBuildGCSResource creates a new BuildGCS resource to pass to knative build
func NewBuildGCSResource(r *PipelineResource) (*BuildGCSResource, error) {
	if r.Spec.Type != PipelineResourceTypeStorage {
		return nil, fmt.Errorf("BuildGCSResource: Cannot create a BuildGCS resource from a %s Pipeline Resource", r.Spec.Type)
	}
	var location, destDir string
	var aType GCSArtifactType
	var locationSpecified, aTypeSpecified bool

	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "Location"):
			location = param.Value
			if param.Value != "" {
				locationSpecified = true
			}
		case strings.EqualFold(param.Name, "DestinationDir"):
			destDir = param.Value
		case strings.EqualFold(param.Name, "ArtifactType"):
			aType = GCSArtifactType(param.Value)
			if param.Value != "" {
				aTypeSpecified = true
			}
		}
	}

	if !locationSpecified {
		return nil, fmt.Errorf("BuildGCSResource: Need Location to be specified in order to create BuildGCS resource %s", r.Name)
	}
	if !aTypeSpecified {
		return nil, fmt.Errorf("BuildGCSResource: Need ArtifactType to be specified in order to fetch BuildGCS resource %s", r.Name)
	}
	return &BuildGCSResource{
		Name:           r.Name,
		Type:           r.Spec.Type,
		Location:       location,
		DestinationDir: destDir,
		ArtifactType:   aType,
		Secrets:        r.Spec.SecretParams,
	}, nil
}

// GetName returns the name of the resource
func (s BuildGCSResource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "storage"
func (s BuildGCSResource) GetType() PipelineResourceType {
	return PipelineResourceTypeStorage
}

// GetParams get params
func (s *BuildGCSResource) GetParams() []Param { return []Param{} }

// GetSecretParams returns the resource secret params
func (s *BuildGCSResource) GetSecretParams() []SecretParam { return s.Secrets }

// GetDownloadContainerSpec returns an array of container specs to download gcs storage object
func (s *BuildGCSResource) GetDownloadContainerSpec() ([]corev1.Container, error) {
	if s.DestinationDir == "" {
		return nil, fmt.Errorf("BuildGCSResource: Expect Destination Directory param to be set %s", s.Name)
	}
	args := []string{"--type", string(s.ArtifactType), "--location", s.Location}
	// dest_dir is the destination directory for GCS files to be copies"
	if s.DestinationDir != "" {
		args = append(args, "--dest_dir", filepath.Join(workspaceDir, s.DestinationDir))
	}

	envVars, secretVolumeMount := getSecretEnvVarsAndVolumeMounts(s.Name, gcsSecretVolumeMountPath, s.Secrets)
	return []corev1.Container{
		CreateDirContainer(s.Name, s.DestinationDir), {
			Name:         fmt.Sprintf("storage-fetch-%s", s.Name),
			Image:        *buildGCSFetcherImage,
			Args:         args,
			Env:          envVars,
			VolumeMounts: secretVolumeMount,
		}}, nil
}

// GetUploadContainerSpec gets container spec for gcs resource to be uploaded like
// set environment variable from secret params and set volume mounts for those secrets
func (s *BuildGCSResource) GetUploadContainerSpec() ([]corev1.Container, error) {
	if s.DestinationDir == "" {
		return nil, fmt.Errorf("BuildGCSResource: Expect Destination Directory param to be set: %s", s.Name)
	}
	args := []string{"--bucket", s.Location}
	envVars, secretVolumeMount := getSecretEnvVarsAndVolumeMounts(s.Name, gcsSecretVolumeMountPath, s.Secrets)

	return []corev1.Container{{
		Name:         fmt.Sprintf("storage-upload-%s", s.Name),
		Image:        *buildGCSUploaderImage,
		Args:         args,
		VolumeMounts: secretVolumeMount,
		Env:          envVars,
	}}, nil
}
