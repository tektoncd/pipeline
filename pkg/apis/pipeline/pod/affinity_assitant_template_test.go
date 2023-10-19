/*
Copyright 2023 The Tekton Authors

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

package pod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestAffinityAssistantTemplate_Equals(t *testing.T) {
	type fields struct {
		NodeSelector     map[string]string
		Tolerations      []corev1.Toleration
		ImagePullSecrets []corev1.LocalObjectReference
	}
	type args struct {
		other *AffinityAssistantTemplate
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   bool
	}{
		{
			name: "test compared obj is nil",
			args: args{
				other: nil,
			},
			want: false,
		},
		{
			name: "test objects are equal",
			fields: fields{
				NodeSelector: map[string]string{
					"test": "test",
				},
				Tolerations: []corev1.Toleration{
					{
						Key:      "test",
						Operator: "==",
						Value:    "test",
					},
				},
				ImagePullSecrets: []corev1.LocalObjectReference{
					{
						Name: "test",
					},
				},
			},
			args: args{
				other: &AffinityAssistantTemplate{
					NodeSelector: map[string]string{
						"test": "test",
					},
					Tolerations: []corev1.Toleration{
						{
							Key:      "test",
							Operator: "==",
							Value:    "test",
						},
					},
					ImagePullSecrets: []corev1.LocalObjectReference{
						{
							Name: "test",
						},
					},
				},
			},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tpl := &AffinityAssistantTemplate{
				NodeSelector:     tt.fields.NodeSelector,
				Tolerations:      tt.fields.Tolerations,
				ImagePullSecrets: tt.fields.ImagePullSecrets,
			}
			if got := tpl.Equals(tt.args.other); got != tt.want {
				t.Errorf("Equals() = %v, want %v", got, tt.want)
			}
		})
	}
}
