/*
Copyright 2018 The Knative Authors.

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
	"flag"

	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	listers "github.com/tektoncd/pipeline/pkg/client/listers/pipeline/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
)

var (
	imageDigestExporterImage = flag.String("imagedigest-exporter-image", "override-with-imagedigest-exporter-image:latest", "The container image containing our image digest exporter binary.")
)

// AddOutputImageDigestExporter add a step to check the index.json for all output images
func AddOutputImageDigestExporter(
	tr *v1alpha1.TaskRun,
	taskSpec *v1alpha1.TaskSpec,
	pipelineResourceLister listers.PipelineResourceLister,
	logger *zap.SugaredLogger,
) error {

	output := []*v1alpha1.ImageResource{}
	if len(tr.Spec.Outputs.Resources) > 0 {
		for _, trb := range tr.Spec.Outputs.Resources {
			boundResource, err := getBoundResource(trb.Name, tr.Spec.Outputs.Resources)
			if err != nil {
				logger.Errorf("Failed to get bound resource: %s", err)
				return err
			}

			resource, err := getResource(boundResource, pipelineResourceLister.PipelineResources(tr.Namespace).Get)
			if err != nil {
				logger.Errorf("Failed to get output pipeline Resource for taskRun %q resource %v; error: %s", tr.Name, boundResource, err.Error())
				return err
			}
			if resource.Spec.Type == v1alpha1.PipelineResourceTypeImage {
				imageResource, err := v1alpha1.NewImageResource(resource)
				if err != nil {
					logger.Errorf("Invalid Image Resource for taskRun %q resource %v; error: %s", tr.Name, boundResource, err.Error())
					return err
				}
				output = append(output, imageResource)
			}
		}

		if len(output) > 0 {
			imagesJSON, err := json.Marshal(output)
			if err != nil {
				return err
			}

			c := corev1.Container{
				Name:    "image-digest-exporter",
				Image:   *imageDigestExporterImage,
				Command: []string{"/ko-app/imagedigestexporter"},
				Args: []string{
					"-images", string(imagesJSON),
				},
			}

			taskSpec.Steps = append(taskSpec.Steps, c)
		}
	}

	return nil
}
