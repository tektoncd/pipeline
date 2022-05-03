/*
Copyright 2022 The Tekton Authors

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

/*
Package common provides constants, errors, labels, annotations and
helpers that are commonly needed by resolvers and clients need during
remote resource resolution.

Ideally this package will never import directly or transitively any
types from kubernetes, knative, tekton pipelines, etc. The intention is
to keep this package tightly focused on shared primitives that are needed
regardless of underlying implementation of resolver or client.
*/
package common
