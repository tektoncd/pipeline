/*
Copyright 2020 The Tekton Authors

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

package config_test

import (
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/pkg/apis/config"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestNewArtifactBucketFromConfigMap(t *testing.T) {
	type testCase struct {
		expectedConfig *config.ArtifactBucket
		fileName       string
	}

	testCases := []testCase{
		{
			expectedConfig: &config.ArtifactBucket{
				Location:                "gs://my-bucket",
				ServiceAccountFieldName: "GOOGLE_APPLICATION_CREDENTIALS",
			},
			fileName: config.GetArtifactBucketConfigName(),
		},
		{
			expectedConfig: &config.ArtifactBucket{
				Location:                 "gs://test-bucket",
				ServiceAccountSecretName: "test-secret",
				ServiceAccountSecretKey:  "key",
				ServiceAccountFieldName:  "some-field",
			},
			fileName: "config-artifact-bucket-all-set",
		},
	}

	for _, tc := range testCases {
		verifyConfigFileWithExpectedArtifactBucketConfig(t, tc.fileName, tc.expectedConfig)
	}
}

func TestNewArtifactBucketFromEmptyConfigMap(t *testing.T) {
	ArtifactBucketConfigEmptyName := "config-artifact-bucket-empty"
	expectedConfig := &config.ArtifactBucket{
		ServiceAccountFieldName: "GOOGLE_APPLICATION_CREDENTIALS",
	}
	verifyConfigFileWithExpectedArtifactBucketConfig(t, ArtifactBucketConfigEmptyName, expectedConfig)
}

func TestGetArtifactBucketConfigName(t *testing.T) {
	for _, tc := range []struct {
		description             string
		artifactsBucketEnvValue string
		expected                string
	}{{
		description:             "Artifact Bucket config value not set",
		artifactsBucketEnvValue: "",
		expected:                "config-artifact-bucket",
	}, {
		description:             "Artifact Bucket config value set",
		artifactsBucketEnvValue: "config-artifact-bucket-test",
		expected:                "config-artifact-bucket-test",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			original := os.Getenv("CONFIG_ARTIFACT_BUCKET_NAME")
			defer t.Cleanup(func() {
				os.Setenv("CONFIG_ARTIFACT_BUCKET_NAME", original)
			})
			if tc.artifactsBucketEnvValue != "" {
				os.Setenv("CONFIG_ARTIFACT_BUCKET_NAME", tc.artifactsBucketEnvValue)
			}
			got := config.GetArtifactBucketConfigName()
			want := tc.expected
			if got != want {
				t.Errorf("GetArtifactBucketConfigName() = %s, want %s", got, want)
			}
		})
	}
}

func verifyConfigFileWithExpectedArtifactBucketConfig(t *testing.T, fileName string, expectedConfig *config.ArtifactBucket) {
	cm := test.ConfigMapFromTestFile(t, fileName)
	if ab, err := config.NewArtifactBucketFromConfigMap(cm); err == nil {
		if d := cmp.Diff(ab, expectedConfig); d != "" {
			t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
		}
	} else {
		t.Errorf("NewArtifactBucketFromConfigMap(actual) = %v", err)
	}
}
