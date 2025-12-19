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

package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// ParseFunc is a function taking ConfigMap data and applying a parse operation to it.
type ParseFunc func(map[string]string) error

func Parse(data map[string]string, parsers ...ParseFunc) error {
	for _, parse := range parsers {
		if err := parse(data); err != nil {
			return err
		}
	}
	return nil
}

func AsFunc[T any](
	key string,
	target *T,
	parseVal func(s string) (T, error),
) ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			val, err := parseVal(raw)
			if err != nil {
				return fmt.Errorf("failed to parse %q: %w", key, err)
			}
			*target = val
		}
		return nil
	}
}

func As[T parseable](key string, target *T) ParseFunc {
	return AsFunc(key, target, parse)
}

type parseable interface {
	int | int16 | int32 | int64 |
		uint | uint16 | uint32 | uint64 |
		string | bool | float64 | float32 |
		time.Duration
}

//nolint:gosec // ignore integer overflow
func parse[T parseable](s string) (T, error) {
	var zero T

	var val any
	var err error

	switch any(zero).(type) {
	case string:
		val = strings.TrimSpace(s)
	case int16:
		val, err = strconv.ParseInt(s, 10, 16)
		val = int16(val.(int64))
	case int32:
		val, err = strconv.ParseInt(s, 10, 32)
		val = int32(val.(int64))
	case int64:
		val, err = strconv.ParseInt(s, 10, 64)
	case uint16:
		val, err = strconv.ParseUint(s, 10, 16)
		val = uint16(val.(uint64))
	case uint32:
		val, err = strconv.ParseUint(s, 10, 32)
		val = uint32(val.(uint64))
	case uint64:
		val, err = strconv.ParseUint(s, 10, 64)
	case float64:
		val, err = strconv.ParseFloat(s, 64)
	case float32:
		val, err = strconv.ParseFloat(s, 64)
		val = float32(val.(float64))
	case bool:
		val, err = strconv.ParseBool(s)
	case time.Duration:
		val, err = time.ParseDuration(s)
	case int:
		val, err = strconv.ParseInt(s, 10, 0)
		val = int(val.(int64))
	case uint:
		val, err = strconv.ParseUint(s, 10, 0)
		val = uint(val.(uint64))
	}

	return val.(T), err
}
