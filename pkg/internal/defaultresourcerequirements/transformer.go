/*
Copyright 2024 The Tekton Authors

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

package defaultresourcerequirements

import (
	"context"
	"strings"

	"github.com/tektoncd/pipeline/pkg/apis/config"
	"github.com/tektoncd/pipeline/pkg/pod"
	corev1 "k8s.io/api/core/v1"
)

// NewTransformer returns a pod.Transformer that will modify container resources if needed
func NewTransformer(ctx context.Context) pod.Transformer {
	// update init container and containers resource requirements
	// resource limits and requests values are taken from a config map
	configDefaults := config.FromContextOrDefaults(ctx).Defaults
	return func(pod *corev1.Pod) (*corev1.Pod, error) {
		return updateResourceRequirements(configDefaults.DefaultContainerResourceRequirements, pod), nil
	}
}

// updates init containers and containers resource requirements of a pod base of config_defaults configmap.
func updateResourceRequirements(resourceRequirementsMap map[string]corev1.ResourceRequirements, pod *corev1.Pod) *corev1.Pod {
	if len(resourceRequirementsMap) == 0 {
		return pod
	}

	// collect all the available container names from the resource requirement map
	// some of the container names: place-scripts, prepare, working-dir-initializer
	// some of the container names with prefix: prefix-scripts, prefix-sidecar-scripts
	containerNames := []string{}
	containerNamesWithPrefix := []string{}
	for containerName := range resourceRequirementsMap {
		// skip the default key
		if containerName == config.ResourceRequirementDefaultContainerKey {
			continue
		}

		if strings.HasPrefix(containerName, "prefix-") {
			containerNamesWithPrefix = append(containerNamesWithPrefix, containerName)
		} else {
			containerNames = append(containerNames, containerName)
		}
	}

	// update the containers resource requirements which does not have resource requirements
	for _, containerName := range containerNames {
		resourceRequirements := resourceRequirementsMap[containerName]
		if resourceRequirements.Size() == 0 {
			continue
		}

		// update init containers
		for index := range pod.Spec.InitContainers {
			targetContainer := pod.Spec.InitContainers[index]
			if containerName == targetContainer.Name && targetContainer.Resources.Size() == 0 {
				pod.Spec.InitContainers[index].Resources = resourceRequirements
			}
		}
		// update containers
		for index := range pod.Spec.Containers {
			targetContainer := pod.Spec.Containers[index]
			if containerName == targetContainer.Name && targetContainer.Resources.Size() == 0 {
				pod.Spec.Containers[index].Resources = resourceRequirements
			}
		}
	}

	// update the containers resource requirements which does not have resource requirements with the mentioned prefix
	for _, containerPrefix := range containerNamesWithPrefix {
		resourceRequirements := resourceRequirementsMap[containerPrefix]
		if resourceRequirements.Size() == 0 {
			continue
		}

		// get actual container name, remove "prefix-" string and append "-" at the end
		// append '-' in the container prefix
		containerPrefix = strings.Replace(containerPrefix, "prefix-", "", 1)
		containerPrefix += "-"

		// update init containers
		for index := range pod.Spec.InitContainers {
			targetContainer := pod.Spec.InitContainers[index]
			if strings.HasPrefix(targetContainer.Name, containerPrefix) && targetContainer.Resources.Size() == 0 {
				pod.Spec.InitContainers[index].Resources = resourceRequirements
			}
		}
		// update containers
		for index := range pod.Spec.Containers {
			targetContainer := pod.Spec.Containers[index]
			if strings.HasPrefix(targetContainer.Name, containerPrefix) && targetContainer.Resources.Size() == 0 {
				pod.Spec.Containers[index].Resources = resourceRequirements
			}
		}
	}

	// reset of the containers resource requirements which has empty resource requirements
	if resourceRequirements, found := resourceRequirementsMap[config.ResourceRequirementDefaultContainerKey]; found && resourceRequirements.Size() != 0 {
		// update init containers
		for index := range pod.Spec.InitContainers {
			if pod.Spec.InitContainers[index].Resources.Size() == 0 {
				pod.Spec.InitContainers[index].Resources = resourceRequirements
			}
		}
		// update containers
		for index := range pod.Spec.Containers {
			if pod.Spec.Containers[index].Resources.Size() == 0 {
				pod.Spec.Containers[index].Resources = resourceRequirements
			}
		}
	}

	return pod
}
