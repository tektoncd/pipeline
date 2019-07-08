/*
Copyright 2019 The Tekton Authors.

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

package config

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	test "github.com/tektoncd/pipeline/pkg/reconciler/testing"
)

func TestNewDefaultsFromConfigMap(t *testing.T) {
	expectedConfig := &Defaults{
		DefaultTimeoutMinutes: 50,
	}
	verifyConfigFileWithExpectedConfig(t, DefaultsConfigName, expectedConfig)
}

func TestNewDefaultsFromEmptyConfigMap(t *testing.T) {
	DefaultsConfigEmptyName := "config-defaults-empty"
	expectedConfig := &Defaults{
		DefaultTimeoutMinutes: 60,
	}
	verifyConfigFileWithExpectedConfig(t, DefaultsConfigEmptyName, expectedConfig)
}

func verifyConfigFileWithExpectedConfig(t *testing.T, fileName string, expectedConfig *Defaults) {
	cm := test.ConfigMapFromTestFile(t, fileName)
	if Defaults, err := NewDefaultsFromConfigMap(cm); err == nil {
		if d := cmp.Diff(Defaults, expectedConfig); d != "" {
			t.Errorf("Diff:\n%s", d)
		}
	} else {
		t.Errorf("NewDefaultsFromConfigMap(actual) = %v", err)
	}
}
