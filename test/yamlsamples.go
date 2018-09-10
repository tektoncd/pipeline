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

package test

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"runtime"

	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	samplePath = "config/samples/"

	PipelineFile = "pipeline_v1beta1_pipeline.yaml"
	PipelineName = "wizzbang-pipeline"

	PipelineParamsFile = "pipeline_v1beta1_pipelineparams.yaml"
	PipelineParamsName = "wizzbang-pipeline-params"

	PipelineRunFile = "pipeline_v1beta1_pipelinerun.yaml"
	PipelineRunName = "wizzbang-pipeline-run-sd8f8dfasdfasdfas"

	TaskFile = "pipeline_v1beta1_task.yaml"
	TaskName = "build-push-task"

	TaskRunFile = "pipeline_v1beta1_taskrun.yaml"
	TaskRunName = "integration-test-wizzbang-task-run-sd8f8dfasdfasdfas"
)

func getSamplePath(fileName string) (string, error) {
	// This is kinda gross: using runtime to see where this file is and then hardcoding
	// paths to the examples based on where this file happens to be at this point in time
	_, filename, _, ok := runtime.Caller(1)
	if !ok {
		return "", fmt.Errorf("")
	}
	return filepath.Join(filename, "../../", samplePath, fileName), nil
}

// DecodeTypeFromYamlSample will read the contents of the yaml file in the samples dir
// called fileName and decode it using objectType, into intoObj.
func DecodeTypeFromYamlSample(fileName string, intoObj k8sruntime.Object) error {
	path, err := getSamplePath(fileName)
	if err != nil {
		return fmt.Errorf("error getting path to sample file %s: %s", fileName, err)
	}
	yaml, err := ioutil.ReadFile(path)
	if err != nil {
		return fmt.Errorf("error reading the file at %s: %s", path, err)
	}

	decode := scheme.Codecs.UniversalDeserializer().Decode
	_, _, err = decode(yaml, nil, intoObj)
	if err != nil {
		return fmt.Errorf("error decoding the file at %s: %s", path, err)
	}
	return nil
}
