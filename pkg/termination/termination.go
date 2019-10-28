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
package termination

import (
	"encoding/json"
	"log"
	"os"

	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
)

func WriteMessage(logger *zap.SugaredLogger, path string, pro []v1alpha1.PipelineResourceResult) {
	jsonOutput, err := json.Marshal(pro)
	if err != nil {
		logger.Fatalf("Error marshaling json: %s", err)
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalf("Unexpected error converting output to json %v: %v", pro, err)
	}
	defer f.Close()

	_, err = f.Write(jsonOutput)
	if err != nil {
		logger.Fatalf("Unexpected error converting output to json %v: %v", pro, err)
	}
	if err := f.Sync(); err != nil {
		logger.Fatalf("Unexpected error converting output to json %v: %v", pro, err)
	}
}
