/*
Copyright 2020 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
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
	"k8s.io/client-go/util/retry"
)

// RetryUpdateConflicts retries the inner function if it returns conflict errors.
// This can be used to retry status updates without constantly reenqueuing keys.
func RetryUpdateConflicts(updater func(int) error) error {
	attempts := 0
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		err := updater(attempts)
		attempts++
		return err
	})
}
