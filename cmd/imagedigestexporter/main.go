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

package main

import (
	"encoding/json"
	"flag"

	"github.com/tektoncd/pipeline/pkg/termination"
	"knative.dev/pkg/logging"

	"github.com/google/go-containerregistry/pkg/v1/layout"
	v1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"github.com/tektoncd/pipeline/pkg/apis/resource/v1alpha1/image"
)

var (
	images                 = flag.String("images", "", "List of images resources built by task in json format")
	terminationMessagePath = flag.String("terminationMessagePath", "/tekton/termination", "Location of file containing termination message")
)

/* The input of this go program will be a JSON string with all the output PipelineResources of type
Image, which will include the path to where the index.json file will be located. The program will
read the related index.json file(s) and log another JSON string including the name of the image resource
and the digests.
The input is an array of ImageResource, ex: [{"name":"srcimg1","type":"image","url":"gcr.io/some-image-1","digest":""}]
The output is an array of PipelineResourceResult, ex: [{"name":"image","digest":"sha256:eed29..660"}]
*/
func main() {
	flag.Parse()
	logger, _ := logging.NewLogger("", "image-digest-exporter")
	defer func() {
		_ = logger.Sync()
	}()

	imageResources := []*image.Resource{}
	if err := json.Unmarshal([]byte(*images), &imageResources); err != nil {
		logger.Fatalf("Error reading images array: %v", err)
	}

	output := []v1beta1.PipelineResourceResult{}
	for _, imageResource := range imageResources {
		ii, err := layout.ImageIndexFromPath(imageResource.OutputImageDir)
		if err != nil {
			logger.Infof("No index.json found for: %s", imageResource.Name)
			continue
		}
		digest, err := GetDigest(ii)
		if err != nil {
			logger.Fatalf("Unexpected error getting image digest for %s: %v", imageResource.Name, err)
		}
		output = append(output, v1beta1.PipelineResourceResult{
			Key:          "digest",
			Value:        digest.String(),
			ResourceName: imageResource.Name,
			ResourceRef: &v1beta1.PipelineResourceRef{
				Name: imageResource.Name,
			},
		})
		output = append(output, v1beta1.PipelineResourceResult{
			Key:          "url",
			Value:        imageResource.URL,
			ResourceName: imageResource.Name,
			ResourceRef: &v1beta1.PipelineResourceRef{
				Name: imageResource.Name,
			},
		})

	}

	if err := termination.WriteMessage(*terminationMessagePath, output); err != nil {
		logger.Fatalf("Unexpected error writing message %s to %s", *terminationMessagePath, err)
	}
}
