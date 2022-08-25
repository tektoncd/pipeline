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

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"strings"

	tektonv1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1"
	tektonv1beta1 "github.com/tektoncd/pipeline/pkg/apis/pipeline/v1beta1"
	"k8s.io/klog"
	"k8s.io/kube-openapi/pkg/common"
	spec "k8s.io/kube-openapi/pkg/validation/spec"
)

func main() {
	pipelinesVersion := flag.String("version", "v0.17.2", "Tekton Pipelines software version")
	apiVersion := flag.String("apiVersion", "v1beta1", "API version")
	flag.Parse()
	version := *pipelinesVersion
	if !strings.HasPrefix(version, "v") {
		version = "v" + version
	}
	var oAPIDefs map[string]common.OpenAPIDefinition
	switch *apiVersion {
	case "v1beta1":
		oAPIDefs = tektonv1beta1.GetOpenAPIDefinitions(func(name string) spec.Ref {
			return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(swaggify(name)))
		})
	case "v1":
		oAPIDefs = tektonv1.GetOpenAPIDefinitions(func(name string) spec.Ref {
			return spec.MustCreateRef("#/definitions/" + common.EscapeJsonPointer(swaggify(name)))
		})
	default:
		panic(fmt.Sprintf("Unsupported API version: %s", *apiVersion))
	}
	defs := spec.Definitions{}
	for defName, val := range oAPIDefs {
		defs[swaggify(defName)] = val.Schema
	}
	swagger := spec.Swagger{
		SwaggerProps: spec.SwaggerProps{
			Swagger:     "2.0",
			Definitions: defs,
			Paths:       &spec.Paths{Paths: map[string]spec.PathItem{}},
			Info: &spec.Info{
				InfoProps: spec.InfoProps{
					Title:       "Tekton",
					Description: "Tekton Pipeline",
					Version:     version,
				},
			},
		},
	}
	jsonBytes, err := json.MarshalIndent(swagger, "", "  ")
	if err != nil {
		klog.Fatal(err.Error())
	}
	fmt.Println(string(jsonBytes))
}

func swaggify(name string) string {
	name = strings.ReplaceAll(name, "./pkg/apis/pipeline/", "")
	name = strings.ReplaceAll(name, "./pkg/apis/resource/", "")
	name = strings.ReplaceAll(name, "github.com/tektoncd/pipeline/pkg/apis/pipeline/", "")
	name = strings.ReplaceAll(name, "github.com/tektoncd/pipeline/pkg/apis/resolution/", "")
	name = strings.ReplaceAll(name, "github.com/tektoncd/pipeline/pkg/apis/resource/", "")
	name = strings.ReplaceAll(name, "k8s.io/api/core/", "")
	name = strings.ReplaceAll(name, "k8s.io/apimachinery/pkg/apis/meta/", "")
	name = strings.ReplaceAll(name, "knative.dev/pkg/apis.", "knative/")
	name = strings.ReplaceAll(name, "knative.dev/pkg/apis/duck/v1beta1.", "knative/")
	name = strings.ReplaceAll(name, "/", ".")
	return name
}
