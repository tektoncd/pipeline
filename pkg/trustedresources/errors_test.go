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

package trustedresources

import (
	"errors"
	"fmt"
	"testing"
)

func TestError(t *testing.T) {
	err := fmt.Errorf("some error")
	verificationErr := VerificationError{err: err, resultType: verificationResultError}
	if verificationErr.Error() != err.Error() {
		t.Errorf("VerificationError Error() want %s got %s", err.Error(), verificationErr.Error())
	}
}

func TestUnwrap(t *testing.T) {
	err := ErrResourceVerificationFailed
	verificationErr := VerificationError{err: err, resultType: verificationResultError}
	unwrap := errors.Unwrap(&verificationErr)
	if !errors.Is(unwrap, err) {
		t.Errorf("VerificationError Unwrap() want %s got %s", err, unwrap)
	}
}

func TestIsVerificationResultError(t *testing.T) {
	tcs := []struct {
		name            string
		verificationErr error
		want            bool
	}{{
		name:            "verificationResultError is verificationResultError",
		verificationErr: &VerificationError{resultType: verificationResultError},
		want:            true,
	}, {
		name:            "verificationResultWarn is not verificationResultError",
		verificationErr: &VerificationError{resultType: verificationResultWarn},
		want:            false,
	}, {
		name:            "verificationResultPass is not verificationResultError",
		verificationErr: &VerificationError{resultType: verificationResultPass},
		want:            false,
	}, {
		name:            "other error is not verificationResultError",
		verificationErr: fmt.Errorf("random error"),
		want:            false,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := IsVerificationResultError(tc.verificationErr)
			if got != tc.want {
				t.Errorf("IsVerificationResultError want:%v got:%v,", tc.want, got)
			}
		})
	}
}

func TestIsVerificationResultPass(t *testing.T) {
	tcs := []struct {
		name            string
		verificationErr error
		want            bool
	}{{
		name:            "verificationResultError is not verificationResultPass",
		verificationErr: &VerificationError{resultType: verificationResultError},
		want:            false,
	}, {
		name:            "verificationResultWarn is not verificationResultPass",
		verificationErr: &VerificationError{resultType: verificationResultWarn},
		want:            false,
	}, {
		name:            "verificationResultPass is not verificationResultPass",
		verificationErr: &VerificationError{resultType: verificationResultPass},
		want:            true,
	}, {
		name:            "other error is not verificationResultError",
		verificationErr: fmt.Errorf("random error"),
		want:            false,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			got := IsVerificationResultPass(tc.verificationErr)
			if got != tc.want {
				t.Errorf("IsVerificationResultPass want:%v got:%v,", tc.want, got)
			}
		})
	}
}
