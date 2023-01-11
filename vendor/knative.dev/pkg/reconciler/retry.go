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
	"strings"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/util/retry"
)

// RetryUpdateConflicts retries the inner function if it returns conflict errors.
// This can be used to retry status updates without constantly reenqueuing keys.
func RetryUpdateConflicts(updater func(int) error) error {
	return RetryErrors(updater, apierrs.IsConflict)
}

// RetryErrors retries the inner function if it returns matching errors.
func RetryErrors(updater func(int) error, fns ...func(error) bool) error {
	attempts := 0
	return retry.OnError(retry.DefaultRetry, func(err error) bool {
		for _, fn := range fns {
			if fn(err) {
				return true
			}
		}
		return false
	}, func() error {
		err := updater(attempts)
		attempts++
		return err
	})
}

// RetryTestErrors retries the inner function if it hits an error type that is
// common in our test environments.
func RetryTestErrors(updater func(int) error) error {
	return RetryErrors(updater,
		// Example: conflicts updating `gke-resource-quotas` (implicit on Service/Pod/Ingress creations)
		apierrs.IsConflict,

		// Example: https://github.com/knative/test-infra/issues/2346#issuecomment-687220045
		func(err error) bool {
			return strings.Contains(err.Error(), "gke-resource-quotas")
		},

		// Example: `etcdserver: request timed out`
		// TODO(mattmoor): Does apierrs.IsServerTimeout catch the above?
		func(err error) bool {
			return strings.Contains(err.Error(), "etcdserver")
		},
	)
}
