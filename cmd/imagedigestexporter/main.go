/*
Copyright 2018 The Tekton Authors

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
)

var (
	images = flag.String("images", "", "List of images resources built by task in json format")
)

/* The input of this go program will be a JSON string with all the output PipelineResources of type
Image, which will include the path to where the index.json file will be located. The program will
read the related index.json file(s) and log another JSON string including the name of the image resource
and the digests.
The input is an array of ImageResource, ex: [{"name":"srcimg1","type":"image","url":"gcr.io/some-image-1","digest":"","OutputImageDir":"/path/image"}]
The output is an array of PipelineResourceResult, ex: [{"name":"image","digest":"sha256:eed29..660"}]
*/
func main() {
	flag.Parse()

	imageResources := []*v1alpha1.ImageResource{}
	err := json.Unmarshal([]byte(*images), &imageResources)
	if err != nil {
		log.Fatalf("Error reading images array: %v", err)
	}

	output := []v1alpha1.PipelineResourceResult{}
	for _, imageResource := range imageResources {
		ii, err := layout.ImageIndexFromPath(imageResource.OutputImageDir)
		if err != nil {
			// if this image doesn't have a builder that supports index.json file,
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
