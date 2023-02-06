/*
 *
 * Copyright 2019 The Tekton Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package v1alpha1_test

import (
	"context"
	"testing"

	"github.com/tektoncd/pipeline/pkg/apis/queue/v1alpha1"
)

func TestValidateQueue(t *testing.T) {
	tcs := []struct {
		name    string
		queue   *v1alpha1.Queue
		wantErr bool
	}{{
		name: "valid cancel",
		queue: &v1alpha1.Queue{
			Spec: v1alpha1.QueueSpec{
				Strategy: "Cancel",
			},
		},
	}, {
		name: "valid gracefully cancel",
		queue: &v1alpha1.Queue{
			Spec: v1alpha1.QueueSpec{
				Strategy: "GracefullyCancel",
			},
		},
	}, {
		name: "valid gracefully stop",
		queue: &v1alpha1.Queue{
			Spec: v1alpha1.QueueSpec{
				Strategy: "GracefullyStop",
			},
		},
	}, {
		name: "no strategy specified",
		queue: &v1alpha1.Queue{
			Spec: v1alpha1.QueueSpec{},
		},
		wantErr: true,
	}, {
		name: "invalid strategy",
		queue: &v1alpha1.Queue{
			Spec: v1alpha1.QueueSpec{
				Strategy: "not-real-strategy",
			},
		},
		wantErr: true,
	}}
	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.queue.Validate(context.Background())
			if (err != nil) != tc.wantErr {
				t.Errorf("wantErr was %t but got error %s", tc.wantErr, err)
			}
		})
	}
}
