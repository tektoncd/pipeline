// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mapper

import "fmt"

type TimerType string

const (
	TimerTypeHistogram TimerType = "histogram"
	TimerTypeSummary   TimerType = "summary"
	TimerTypeDefault   TimerType = ""
)

func (t *TimerType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}

	switch TimerType(v) {
	case TimerTypeHistogram:
		*t = TimerTypeHistogram
	case TimerTypeSummary, TimerTypeDefault:
		*t = TimerTypeSummary
	default:
		return fmt.Errorf("invalid timer type '%s'", v)
	}
	return nil
}
