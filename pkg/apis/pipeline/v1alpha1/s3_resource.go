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
	"flag"
	"fmt"
	"path"
	"strings"

	"github.com/tektoncd/pipeline/pkg/names"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
)

var (
	s3utilImage             = flag.String("s3-image", "override-with-s3-image:latest", "The container image containing aws-cli")
	s3SecretVolumeMountPath = "/var/secret"
)

// S3Resource represents an object in an s3 bucket, which we can get(input) or put(output).
type S3Resource struct {
	Name       string               `json:"name"`
	BucketName string               `json:"bucketname"`
	ObjectName string               `json:"objectname"`
	Location   string               `json:"location"`
	Type       PipelineResourceType `json:"type"`
	Secrets    []SecretParam        `json:"secrets"`
}

// NewS3Resource creates a new S3 resource to pass to a Task
func NewS3Resource(r *PipelineResource) (*S3Resource, error) {
	if r.Spec.Type != PipelineResourceTypeStorage {
		return nil, xerrors.Errorf("S3: Cannot create a S3 resource from a %s Pipeline Resource", r.Spec.Type)
	}
	var location string
	var bucket string
	var object string
	var locationSpecified, objectSpecified, bucketSpecified bool

	for _, param := range r.Spec.Params {
		switch {
		case strings.EqualFold(param.Name, "Location"):
			location = param.Value
			if param.Value != "" {
				locationSpecified = true
			}
		case strings.EqualFold(param.Name, "Bucket"):
			bucket = param.Value
			if param.Value != "" {
				bucketSpecified = true
			}
		case strings.EqualFold(param.Name, "ObjectName"):
			object = param.Value
			if param.Value != "" {
				objectSpecified = true
			}
		}
	}

	if !locationSpecified {
		return nil, xerrors.Errorf("S3Resource: Need Location to be specified in order to create S3 resource %s", r.Name)
	}
	if !bucketSpecified {
		return nil, xerrors.Errorf("S3Resource: Need Bucket to be specified in order to create S3 resource %s", r.Name)
	}
	if !objectSpecified {
		return nil, xerrors.Errorf("S3Resource: Need Object to be specified in order to create S3 resource %s", r.Name)
	}

	return &S3Resource{
		Name:       r.Name,
		BucketName: bucket,
		ObjectName: object,
		Type:       r.Spec.Type,
		Location:   location,
		Secrets:    r.Spec.SecretParams,
	}, nil
}

// GetName returns the name of the resource
func (s *S3Resource) GetName() string {
	return s.Name
}

// GetType returns the type of the resource, in this case "storage"
func (s *S3Resource) GetType() PipelineResourceType {
	return PipelineResourceTypeStorage
}

// GetSecretParams returns the resource secret params
func (s *S3Resource) GetSecretParams() []SecretParam { return s.Secrets }

// GetUploadSteps gets container spec for s3 resource to be uploaded
func (s *S3Resource) GetDownloadSteps(sourcePath string) ([]Step, error) {
	args := []string{"--put", "--destination", path.Join(sourcePath, s.Location), "--artifact", s.ObjectName, "--bucket", s.BucketName}

	envVars, secretVolumeMount := getSecretEnvVarsAndVolumeMounts(s.Name, s3SecretVolumeMountPath, s.Secrets)

	return []Step{{Container: corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("upload-%s", s.Name)),
		Image:        *s3utilImage,
		Command:      []string{"/ko-app/s3store"},
		Args:         args,
		VolumeMounts: secretVolumeMount,
		Env:          envVars,
	}}}, nil
}

func (s *S3Resource) GetUploadSteps(sourcePath string) ([]Step, error) {
	args := []string{"--put", "--source", path.Join(sourcePath, s.Location), "--artifact", s.ObjectName, "--bucket", s.BucketName}

	envVars, secretVolumeMount := getSecretEnvVarsAndVolumeMounts(s.Name, s3SecretVolumeMountPath, s.Secrets)

	return []Step{{Container: corev1.Container{
		Name:         names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(fmt.Sprintf("upload-%s", s.Name)),
		Image:        *s3utilImage,
		Command:      []string{"/ko-app/s3store"},
		Args:         args,
		VolumeMounts: secretVolumeMount,
		Env:          envVars,
	}}}, nil

}

func (s *S3Resource) GetUploadVolumeSpec(spec *TaskSpec) ([]corev1.Volume, error) {
	return getStorageVolumeSpec(s, spec)
}

func (s *S3Resource) GetDownloadVolumeSpec(spec *TaskSpec) ([]corev1.Volume, error) {
	return getStorageVolumeSpec(s, spec)
}

// Replacements is used for template replacement on an S3Resource inside of a Taskrun.
func (s *S3Resource) Replacements() map[string]string {
	return map[string]string{
		"name":       s.Name,
		"bucketname": s.BucketName,
		"objectname": s.ObjectName,
	}
}
