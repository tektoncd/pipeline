/*
Copyright 2020 The Knative Authors

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

package spoof

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/sets"
)

// MatchesBody checks that the *first* response body matches the "expected" body, otherwise failing.
func MatchesBody(expected string) ResponseChecker {
	return func(resp *Response) (bool, error) {
		if !strings.Contains(string(resp.Body), expected) {
			// Returning (true, err) causes SpoofingClient.Poll to fail.
			return true, fmt.Errorf("body = %s, want: %s", string(resp.Body), expected)
		}

		return true, nil
	}
}

// MatchesAllOf combines multiple ResponseCheckers to one ResponseChecker with a logical AND. The
// checkers are executed in order. The first function to trigger an error or a retry will short-circuit
// the other functions (they will not be executed).
//
// This is useful for combining a body with a status check like:
// MatchesAllOf(IsStatusOK, MatchesBody("test"))
//
// The MatchesBody check will only be executed after the IsStatusOK has passed.
func MatchesAllOf(checkers ...ResponseChecker) ResponseChecker {
	return func(resp *Response) (bool, error) {
		for _, checker := range checkers {
			if done, err := checker(resp); err != nil || !done {
				return done, err
			}
		}
		return true, nil
	}
}

// MatchesAllBodies checks that the *first* response body matches the "expected" body, otherwise failing.
func MatchesAllBodies(all ...string) ResponseChecker {
	var m sync.Mutex
	// This helps with two things:
	// 1. we can use Equal on sets
	// 2. it will collapse the duplicates
	want := sets.NewString(all...)
	seen := make(sets.String, len(all))

	return func(resp *Response) (bool, error) {
		bs := string(resp.Body)
		for expected := range want {
			if !strings.Contains(bs, expected) {
				// See if the next one matches.
				continue
			}

			m.Lock()
			defer m.Unlock()
			seen.Insert(expected)

			// Stop once we've seen them all.
			return want.Equal(seen), nil
		}

		// Returning (true, err) causes SpoofingClient.Poll to fail.
		return true, fmt.Errorf("body = %s, want one of: %s", bs, all)
	}
}

// IsStatusOK checks that the response code is a 200.
func IsStatusOK(resp *Response) (bool, error) {
	return IsOneOfStatusCodes(http.StatusOK)(resp)
}

// IsOneOfStatusCodes checks that the response code is equal to the given one.
func IsOneOfStatusCodes(codes ...int) ResponseChecker {
	return func(resp *Response) (bool, error) {
		for _, code := range codes {
			if resp.StatusCode == code {
				return true, nil
			}
		}

		return true, fmt.Errorf("status = %d %s, want one of: %v", resp.StatusCode, resp.Status, codes)
	}
}
