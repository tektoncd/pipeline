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

// Package duck defines logic for defining and consuming "duck typed"
// Kubernetes resources.  Producers define partial resource definitions
// that resource authors may choose to implement to interoperate with
// consumers of these "duck typed" interfaces.
// For more information see:
// TODO(mattmoor): Add link to doc.
package duck
