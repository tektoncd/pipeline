// Copyright © 2019 The Tekton Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package labels

import (
	"reflect"
	"testing"

	"github.com/tektoncd/cli/pkg/test"
)

func Test_MergeLabels(t *testing.T) {

	Labels := map[string]string{"alatouki": "lamaracarena"}

	// invalid
	_, err := MergeLabels(Labels, []string{"test"})
	if err == nil {
		t.Errorf("Expected error")
	}
	test.AssertOutput(t, 1, len(Labels))

	// add
	Labels, err = MergeLabels(Labels, []string{"label1=test"})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 2, len(Labels))

	Labels, err = MergeLabels(Labels, []string{})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 2, len(Labels))

	// update
	Labels, err = MergeLabels(Labels, []string{"alatouki=paslamacarena"})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 2, len(Labels))
	test.AssertOutput(t, "paslamacarena", Labels["alatouki"])

	// multiples
	Labels, err = MergeLabels(Labels, []string{"label2=test2", "label3=label3"})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 4, len(Labels))

	Labels, err = MergeLabels(nil, []string{"label2=test2", "label3=label3"})
	if err != nil {
		t.Errorf("Did not expect error")
	}
	test.AssertOutput(t, 2, len(Labels))
}

func Test_parseLabels(t *testing.T) {
	type args struct {
		p []string
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]string
		wantErr bool
	}{{
		name: "Test_parseMerge No Err",
		args: args{
			p: []string{"key1=value1", "key2=value2", "key3=value3"},
		},
		want: map[string]string{
			"key1": "value1",
			"key2": "value2",
			"key3": "value3",
		},
		wantErr: false,
	}, {
		name: "Test_parseLabel Err",
		args: args{
			p: []string{"value1", "value2"},
		},
		wantErr: true,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLabels(tt.args.p)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseLabels() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseLabels() = %v, want %v", got, tt.want)
			}
		})
	}
}
