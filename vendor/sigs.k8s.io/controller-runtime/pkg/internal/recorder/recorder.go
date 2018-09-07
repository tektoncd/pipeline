/*
Copyright 2018 The Kubernetes Authors.

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

package recorder

import (
	"fmt"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/recorder"
)

type provider struct {
	// scheme to specify when creating a recorder
	scheme *runtime.Scheme
	// eventBroadcaster to create new recorder instance
	eventBroadcaster record.EventBroadcaster
}

// NewProvider create a new Provider instance.
func NewProvider(config *rest.Config, scheme *runtime.Scheme) (recorder.Provider, error) {
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to init clientSet: %v", err)
	}

	p := &provider{scheme: scheme}
	p.eventBroadcaster = record.NewBroadcaster()
	p.eventBroadcaster.StartLogging(glog.Infof)
	p.eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientSet.CoreV1().Events("")})

	return p, nil
}

func (p *provider) GetEventRecorderFor(name string) record.EventRecorder {
	return p.eventBroadcaster.NewRecorder(p.scheme, corev1.EventSource{Component: name})
}
