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

package artifacts

import (
	"fmt"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline"
	v1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	corev1 "k8s.io/api/core/v1"
)

// ArtifactDownloadAndVerify is the template script that downloads and verifies artifact in the injected step
const ArtifactDownloadAndVerify = `#!/usr/bin/env bash
set -e
names=($ARTIFACT_NAMES)
paths=($ARTIFACT_PATHS)
hashes=($ARTIFACT_HASHES)
types=($ARTIFACT_TYPES)
ARTIFACTS_ROOT="${ARTIFACT_WORKSPACE_PATH}/.tekton/artifacts"
TARGET_ROOT="${ARTIFACT_LOCAL_PATH}"
if [ ! -d "$ARTIFACTS_ROOT" ]; then
	>&2 echo "Artifact folder $ARTIFACTS_ROOT missing"
	exit 1
fi
for i in ${!names[@]}; do
	ext=""
	if [ "${types[$i]}" == "folder" ]; then
		ext=".tgz"
	fi
	ARTIFACT="${ARTIFACTS_ROOT}/${hashes[$i]}${ext}"
	if [ ! -f "$ARTIFACT" ]; then
		>&2 echo "Artifact "${names[$i]}" missing @ ${ARTIFACT}"
		exit 1
	fi
	TARGET_ARTIFACT="${TARGET_ROOT}/${paths[$i]}${ext}"
	cp "$ARTIFACT" "$TARGET_ARTIFACT"
	echo "${hashes[$i]} ${TARGET_ARTIFACT}" | md5sum -c || ret=$?
	if [[ $ret -ne 0 ]]; then
		>&2 echo "md5sum of ${names[$i]} doesn't match. Expected ${hashes[$i]}, found $(md5sum ${TARGET_ARTIFACT})"
		exit 1
	fi
	if [ "${types[$i]}" == "folder" ]; then
		TARGET_FOLDER="${TARGET_ROOT}/${paths[$i]}"
		mkdir -p "$TARGET_FOLDER" || echo "warning: target folder $TARGET_FOLDER already exists for artifact ${names[$i]}"
		tar zxf "${TARGET_ARTIFACT}" -C "${TARGET_FOLDER}"
		rm "${TARGET_ARTIFACT}"
	fi
done
`

// GetArtifactStep produces a step the downloads all artifacts defined in params, from the
// workspace artifact to the local artifact emptyDir, and validates the hash for all of them
func GetArtifactStep(workspaces []v1.WorkspaceDeclaration, params v1.ParamSpecs) (*v1.Step, error) {
	var artifactWS *v1.WorkspaceDeclaration
	// Look for an artifact workspace first
	for _, ws := range workspaces {
		if ws.Artifact {
			artifactWS = ws.DeepCopy()
			// Only one exists, enforced via validation
			break
		}
	}
	// Loop all params, process any artifact one
	var names, paths, hashes, types []string
	for _, param := range params {
		if param.Type == v1.ParamTypeArtifact {
			if artifactWS == nil {
				return nil, fmt.Errorf("param %s is of type artifact, but no artifact workspace was found", param.Name)
			}
			names = append(names, param.Name)
			paths = append(paths, fmt.Sprintf("$(params.%s.path)", param.Name))
			hashes = append(hashes, fmt.Sprintf("$(params.%s.hash)", param.Name))
			types = append(types, fmt.Sprintf("$(params.%s.type)", param.Name))
		}
	}
	// No artifact param found, no steps injected
	if len(names) == 0 {
		return nil, nil
	}
	downloadStep := v1.Step{
		Name:  "tekton-artifact-download",
		Image: "bash:latest", // TODO(afrittoli) Make this configurable
		Env: []corev1.EnvVar{{
			Name:  "ARTIFACT_NAMES",
			Value: strings.Join(names, " "),
		}, {
			Name:  "ARTIFACT_PATHS",
			Value: strings.Join(paths, " "),
		}, {
			Name:  "ARTIFACT_HASHES",
			Value: strings.Join(hashes, " "),
		}, {
			Name:  "ARTIFACT_TYPES",
			Value: strings.Join(types, " "),
		}, {
			Name:  "ARTIFACT_WORKSPACE_PATH",
			Value: artifactWS.GetMountPath(),
		}, {
			Name:  "ARTIFACT_LOCAL_PATH",
			Value: pipeline.ArtifactsDir,
		}},
		Script: ArtifactDownloadAndVerify,
	}
	return &downloadStep, nil
}
