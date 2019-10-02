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
	"flag"

	"knative.dev/pkg/injection/sharedmain"

	"github.com/tektoncd/pipeline/pkg/reconciler"
	"github.com/tektoncd/pipeline/pkg/reconciler/pipelinerun"
	"github.com/tektoncd/pipeline/pkg/reconciler/taskrun"
)

const (
	// ControllerLogKey is the name of the logger for the controller cmd
	ControllerLogKey = "controller"
)

var (
	entrypointImage = flag.String("entrypoint-image", "override-with-entrypoint:latest",
		"The container image containing our entrypoint binary.")
	nopImage = flag.String("nop-image", "override-with-nop:latest",
		"The container image used to kill sidecars")
)

func main() {
	flag.Parse()
	images := reconciler.Images{
		EntryPointImage: *entrypointImage,
		NopImage:        *nopImage,
	}
	sharedmain.Main(ControllerLogKey,
		taskrun.NewController(images),
		pipelinerun.NewController(images),
	)
}
