/*
Copyright 2024 The Tekton Authors

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
Package resolver contains the upgraded remote resolution framework.
It contains the upgraded framework and the built-in resolves.
It is equivalent to `pkg/resolution/resolver`.
This was necessary to ensure backwards compatibility with the existing framework.

This package is subject to further refactoring and changes.
*/
package resolver
