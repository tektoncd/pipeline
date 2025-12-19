/*
Copyright 2025 The Knative Authors

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

package attributekey

import (
	"fmt"

	"go.opentelemetry.io/otel/attribute"
)

type ValueType interface {
	string | bool | int64 | int | float64 |
		[]string | []bool | []int64 | []int | []float64
}

type (
	Type[T ValueType] string
	String            = Type[string]
	Bool              = Type[bool]
	Int               = Type[int]
	Int64             = Type[int64]
	Float64           = Type[float64]
)

func (key Type[T]) With(val T) attribute.KeyValue {
	k := string(key)

	switch v := any(val).(type) {
	case string:
		return attribute.String(k, v)
	case bool:
		return attribute.Bool(k, v)
	case int64:
		return attribute.Int64(k, v)
	case int:
		return attribute.Int(k, v)
	case float64:
		return attribute.Float64(k, v)
	case []string:
		return attribute.StringSlice(k, v)
	case []bool:
		return attribute.BoolSlice(k, v)
	case []int64:
		return attribute.Int64Slice(k, v)
	case []int:
		return attribute.IntSlice(k, v)
	case []float64:
		return attribute.Float64Slice(k, v)
	default:
		// note - this can't happen due to type constraints
		panic(fmt.Sprintf("unsupported attribute type: %T", v))
	}
}
