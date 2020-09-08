// Copyright 2018 The Prometheus Authors
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

type MetricType string

const (
	MetricTypeCounter MetricType = "counter"
	MetricTypeGauge   MetricType = "gauge"
	MetricTypeTimer   MetricType = "timer"
)

func (m *MetricType) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var v string
	if err := unmarshal(&v); err != nil {
		return err
	}

	switch MetricType(v) {
	case MetricTypeCounter:
		*m = MetricTypeCounter
	case MetricTypeGauge:
		*m = MetricTypeGauge
	case MetricTypeTimer:
		*m = MetricTypeTimer
	default:
		return fmt.Errorf("invalid metric type '%s'", v)
	}
	return nil
}
