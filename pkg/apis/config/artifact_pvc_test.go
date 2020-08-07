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

func TestNewArtifactPVCFromConfigMap(t *testing.T) {
	type testCase struct {
		expectedConfig *config.ArtifactPVC
		fileName       string
	}

	testCases := []testCase{
		{
			expectedConfig: &config.ArtifactPVC{
				Size:             "5Gi",
				StorageClassName: "my-class",
			},
			fileName: config.GetArtifactPVCConfigName(),
		},
		{
			expectedConfig: &config.ArtifactPVC{
				Size:             "10Gi",
				StorageClassName: "test-class",
			},
			fileName: "config-artifact-pvc-all-set",
		},
	}

	for _, tc := range testCases {
		verifyConfigFileWithExpectedArtifactPVCConfig(t, tc.fileName, tc.expectedConfig)
	}
}

func TestNewArtifactPVCFromEmptyConfigMap(t *testing.T) {
	ArtifactPVCConfigEmptyName := "config-artifact-pvc-empty"
	expectedConfig := &config.ArtifactPVC{
		Size: "5Gi",
	}
	verifyConfigFileWithExpectedArtifactPVCConfig(t, ArtifactPVCConfigEmptyName, expectedConfig)
}

func TestGetArtifactPVCConfigName(t *testing.T) {
	for _, tc := range []struct {
		description          string
		artifactsPVCEnvValue string
		expected             string
	}{{
		description:          "Artifact PVC config value not set",
		artifactsPVCEnvValue: "",
		expected:             "config-artifact-pvc",
	}, {
		description:          "Artifact PVC config value set",
		artifactsPVCEnvValue: "config-artifact-pvc-test",
		expected:             "config-artifact-pvc-test",
	}} {
		t.Run(tc.description, func(t *testing.T) {
			original := os.Getenv("CONFIG_ARTIFACT_PVC_NAME")
			defer t.Cleanup(func() {
				os.Setenv("CONFIG_ARTIFACT_PVC_NAME", original)
			})
			if tc.artifactsPVCEnvValue != "" {
				os.Setenv("CONFIG_ARTIFACT_PVC_NAME", tc.artifactsPVCEnvValue)
			}
			got := config.GetArtifactPVCConfigName()
			want := tc.expected
			if got != want {
				t.Errorf("GetArtifactPVCConfigName() = %s, want %s", got, want)
			}
		})
	}
}

func verifyConfigFileWithExpectedArtifactPVCConfig(t *testing.T, fileName string, expectedConfig *config.ArtifactPVC) {
	cm := test.ConfigMapFromTestFile(t, fileName)
	if ab, err := config.NewArtifactPVCFromConfigMap(cm); err == nil {
		if d := cmp.Diff(ab, expectedConfig); d != "" {
			t.Errorf("Diff:\n%s", diff.PrintWantGot(d))
		}
	} else {
		t.Errorf("NewArtifactPVCFromConfigMap(actual) = %v", err)
	}
}
