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

package controller

import (
	"go.uber.org/zap"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"

	"github.com/knative/build/pkg/builder"

	clientset "github.com/knative/build/pkg/client/clientset/versioned"
	buildscheme "github.com/knative/build/pkg/client/clientset/versioned/scheme"
	informers "github.com/knative/build/pkg/client/informers/externalversions"
)

func init() {
	// Add build types to the default Kubernetes Scheme so Events can be
	// logged for build types.
	buildscheme.AddToScheme(scheme.Scheme)
}

// Interface is the interface of a controller.
type Interface interface {
	Run(threadiness int, stopCh <-chan struct{}) error
}

// Constructor defines the method signature for a controller constructor.
type Constructor func(
	builder.Interface,
	kubernetes.Interface,
	clientset.Interface,
	kubeinformers.SharedInformerFactory,
	informers.SharedInformerFactory,
	*zap.SugaredLogger,
) Interface
