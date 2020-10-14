/*
Copyright 2019 The Knative Authors

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

package helpers

import (
	"log"
)

// Run can run functions that needs dryrun support.
func Run(message string, call func() error, dryrun bool) error {
	if dryrun {
		log.Printf("[dry run] %s", message)
		return nil
	}
	log.Print(message)

	return call()
}
