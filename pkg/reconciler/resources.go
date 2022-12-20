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

package reconciler

import (
	"time"

	"knative.dev/pkg/kmeta"
)

const (
	// minimumResourceAge is the age at which resources stop being IsYoungResource.
	minimumResourceAge = 5 * time.Second
)

// IsYoungResource checks whether the resource is younger than minimumResourceAge, based on its creation timestamp.
func IsYoungResource(obj kmeta.Accessor) bool {
	return time.Since(obj.GetCreationTimestamp().Time) < minimumResourceAge
}
