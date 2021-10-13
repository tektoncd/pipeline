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

package config

import (
	"os"

	corev1 "k8s.io/api/core/v1"
)

const (
	// BucketLocationKey is the name of the configmap entry that specifies
	// loction of the bucket.
	BucketLocationKey = "location"

	// BucketServiceAccountSecretNameKey is the name of the configmap entry that specifies
	// the name of the secret that will provide the servie account with bucket access.
	// This secret must  have a key called serviceaccount that will have a value with
	// the service account with access to the bucket
	BucketServiceAccountSecretNameKey = "bucket.service.account.secret.name"

	// BucketServiceAccountSecretKeyKey is the name of the configmap entry that specifies
	// the secret key that will have a value with the service account json with access
	// to the bucket
	BucketServiceAccountSecretKeyKey = "bucket.service.account.secret.key"

	// DefaultBucketServiceFieldName defaults to a gcs bucket
	DefaultBucketServiceFieldName = "GOOGLE_APPLICATION_CREDENTIALS"

	// BucketServiceAccountFieldNameKey is the name of the configmap entry that specifies
	// the field name that should be used for the service account.
	// Valid values: GOOGLE_APPLICATION_CREDENTIALS, BOTO_CONFIG.
	BucketServiceAccountFieldNameKey = "bucket.service.account.field.name"
)

// ArtifactBucket holds the configurations for the artifacts PVC
// +k8s:deepcopy-gen=true
type ArtifactBucket struct {
	Location                 string
	ServiceAccountSecretName string
	ServiceAccountSecretKey  string
	ServiceAccountFieldName  string
}

// GetArtifactBucketConfigName returns the name of the configmap containing all
// customizations for the storage bucket.
func GetArtifactBucketConfigName() string {
	if e := os.Getenv("CONFIG_ARTIFACT_BUCKET_NAME"); e != "" {
		return e
	}
	return "config-artifact-bucket"
}

// Equals returns true if two Configs are identical
func (cfg *ArtifactBucket) Equals(other *ArtifactBucket) bool {
	if cfg == nil && other == nil {
		return true
	}

	if cfg == nil || other == nil {
		return false
	}

	return other.Location == cfg.Location &&
		other.ServiceAccountSecretName == cfg.ServiceAccountSecretName &&
		other.ServiceAccountSecretKey == cfg.ServiceAccountSecretKey &&
		other.ServiceAccountFieldName == cfg.ServiceAccountFieldName
}

// NewArtifactBucketFromMap returns a Config given a map corresponding to a ConfigMap
func NewArtifactBucketFromMap(cfgMap map[string]string) (*ArtifactBucket, error) {
	tc := ArtifactBucket{
		ServiceAccountFieldName: DefaultBucketServiceFieldName,
	}

	if location, ok := cfgMap[BucketLocationKey]; ok {
		tc.Location = location
	}

	if serviceAccountSecretName, ok := cfgMap[BucketServiceAccountSecretNameKey]; ok {
		tc.ServiceAccountSecretName = serviceAccountSecretName
	}

	if serviceAccountSecretKey, ok := cfgMap[BucketServiceAccountSecretKeyKey]; ok {
		tc.ServiceAccountSecretKey = serviceAccountSecretKey
	}

	if serviceAccountFieldName, ok := cfgMap[BucketServiceAccountFieldNameKey]; ok {
		tc.ServiceAccountFieldName = serviceAccountFieldName
	}

	return &tc, nil
}

// NewArtifactBucketFromConfigMap returns a Config for the given configmap
func NewArtifactBucketFromConfigMap(config *corev1.ConfigMap) (*ArtifactBucket, error) {
	return NewArtifactBucketFromMap(config.Data)
}
