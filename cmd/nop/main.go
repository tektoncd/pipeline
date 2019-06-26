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

// The nop command is a no-op, it simply prints a message and exits. Nop
// is used to stop sidecar containers in TaskRun Pods. When a Task's Steps
// are complete any sidecars running alongside the Step containers need
// to be terminated. Whatever image the sidecars are running is replaced
// with nop and the sidecar quickly exits.

package main

import "fmt"

func main() {
	fmt.Println("Task completed successfully")
}
