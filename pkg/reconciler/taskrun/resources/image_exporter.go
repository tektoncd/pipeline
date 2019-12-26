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

package resources

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"github.com/tektoncd/pipeline/pkg/names"
	corev1 "k8s.io/api/core/v1"
)

const imageDigestExporterContainerName = "image-digest-exporter"

// AddOutputImageDigestExporter add a step to check the index.json for all output images
func AddOutputImageDigestExporter(
	imageDigestExporterImage string,
	tr *v1alpha1.TaskRun,
	taskSpec *v1alpha1.TaskSpec,
	gr GetResource,
) error {

	output := []*v1alpha1.ImageResource{}
	if len(tr.Spec.Outputs.Resources) > 0 {
		for _, trb := range tr.Spec.Outputs.Resources {
			boundResource, err := getBoundResource(trb.Name, tr.Spec.Outputs.Resources)
			if err != nil {
				return fmt.Errorf("failed to get bound resource: %w while adding output image digest exporter", err)
			}

			resource, err := GetResourceFromBinding(&boundResource.PipelineResourceBinding, gr)
			if err != nil {
				return fmt.Errorf("failed to get output pipeline Resource for taskRun %q resource %v; error: %w while adding output image digest exporter", tr.Name, boundResource, err)
			}
			if resource.Spec.Type == v1alpha1.PipelineResourceTypeImage {
				imageResource, err := v1alpha1.NewImageResource(resource)
				if err != nil {
					return fmt.Errorf("invalid Image Resource for taskRun %q resource %v; error: %w", tr.Name, boundResource, err)
				}
				for _, o := range taskSpec.Outputs.Resources {
					if o.Name == boundResource.Name {
						if o.TargetPath == "" {
							imageResource.OutputImageDir = filepath.Join(outputDir, boundResource.Name)
						} else {
							imageResource.OutputImageDir = o.TargetPath
						}
						break
					}
				}
				output = append(output, imageResource)
			}
		}

		if len(output) > 0 {
			augmentedSteps := []v1alpha1.Step{}
			imagesJSON, err := json.Marshal(output)
			if err != nil {
				return fmt.Errorf("failed to format image resource data for output image exporter: %w", err)
			}

			augmentedSteps = append(augmentedSteps, taskSpec.Steps...)
			augmentedSteps = append(augmentedSteps, imageDigestExporterStep(imageDigestExporterImage, imagesJSON))

			taskSpec.Steps = augmentedSteps
		}

	}

	return nil
}

func imageDigestExporterStep(imageDigestExporterImage string, imagesJSON []byte) v1alpha1.Step {
	return v1alpha1.Step{Container: corev1.Container{
		Name:    names.SimpleNameGenerator.RestrictLengthWithRandomSuffix(imageDigestExporterContainerName),
		Image:   imageDigestExporterImage,
		Command: []string{"/ko-app/imagedigestexporter"},
		Args: []string{
			"-images", string(imagesJSON),
		},
		TerminationMessagePolicy: corev1.TerminationMessageFallbackToLogsOnError,
	}}
}
