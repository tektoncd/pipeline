/*
Copyright 2018 The Knative Authors

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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	v1alpha1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
)

var (
	images = flag.String("images", "", "List of images resources built by task in json format")
)

func main() {
	flag.Parse()

	imageResources := []*v1alpha1.ImageResource{}
	err := json.Unmarshal([]byte(*images), &imageResources)
	if err != nil {
		log.Fatalf("Error reading images array: %v", err)
	}

	output := []v1alpha1.PipelineResourceResult{}
	for _, imageResource := range imageResources {
		ii, err := layout.ImageIndexFromPath(imageResource.IndexPath)
		if err != nil {
			// if this image doesn't have a builder that supports index.josn file,
			// then it will be skipped
			continue
		}
		digest, err := ii.Digest()
		if err != nil {
			log.Fatalf("Unexpected error getting image digest %v: %v", imageResource, err)
		}
		output = append(output, v1alpha1.PipelineResourceResult{Name: imageResource.Name, Digest: digest.String()})
	}

	imagesJSON, err := json.Marshal(output)
	if err != nil {
		log.Fatalf("Unexpected error converting images to json %v: %v", output, err)
	}
	fmt.Println(string(imagesJSON))
}
