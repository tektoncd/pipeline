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

package configmap

import (
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/validation"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"knative.dev/pkg/configmap/parser"
)

// ParseFunc is a function taking ConfigMap data and applying a parse operation to it.
type ParseFunc = parser.ParseFunc

// AsString passes the value at key through into the target, if it exists.
var AsString = parser.As[string]

// AsBool parses the value at key as a boolean into the target, if it exists.
var AsBool = parser.As[bool]

// AsInt16 parses the value at key as an int16 into the target, if it exists.
var AsInt16 = parser.As[int16]

// AsInt32 parses the value at key as an int32 into the target, if it exists.
var AsInt32 = parser.As[int32]

// AsInt64 parses the value at key as an int64 into the target, if it exists.
var AsInt64 = parser.As[int64]

// AsInt parses the value at key as an int into the target, if it exists.
var AsInt = parser.As[int]

// AsUint16 parses the value at key as an uint16 into the target, if it exists.
var AsUint16 = parser.As[uint16]

// AsUint32 parses the value at key as an uint32 into the target, if it exists.
var AsUint32 = parser.As[uint32]

// AsUint64 parses the value at key as an uint32 into the target, if it exists.
var AsUint64 = parser.As[uint32]

// AsFloat64 parses the value at key as a float64 into the target, if it exists.
var AsFloat64 = parser.As[float64]

// AsDuration parses the value at key as a time.Duration into the target, if it exists.
var AsDuration = parser.As[time.Duration]

// AsStringSet parses the value at key as a sets.Set[string] (split by ',') into the target, if it exists.
func AsStringSet(key string, target *sets.Set[string]) ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			splitted := strings.Split(raw, ",")
			for i, v := range splitted {
				splitted[i] = strings.TrimSpace(v)
			}
			*target = sets.New[string](splitted...)
		}
		return nil
	}
}

// AsQuantity parses the value at key as a *resource.Quantity into the target, if it exists
func AsQuantity(key string, target **resource.Quantity) ParseFunc {
	return func(data map[string]string) error {
		if raw, ok := data[key]; ok {
			val, err := resource.ParseQuantity(raw)
			if err != nil {
				return fmt.Errorf("failed to parse %q: %w", key, err)
			}

			*target = &val
		}
		return nil
	}
}

// AsOptionalNamespacedName parses the value at key as a types.NamespacedName into the target, if it exists
// The namespace and name are both required and expected to be valid DNS labels
func AsOptionalNamespacedName(key string, target **types.NamespacedName) ParseFunc {
	return func(data map[string]string) error {
		if _, ok := data[key]; !ok {
			return nil
		}

		*target = &types.NamespacedName{}
		return AsNamespacedName(key, *target)(data)
	}
}

// AsNamespacedName parses the value at key as a types.NamespacedName into the target, if it exists
// The namespace and name are both required and expected to be valid DNS labels
func AsNamespacedName(key string, target *types.NamespacedName) ParseFunc {
	return func(data map[string]string) error {
		raw, ok := data[key]
		if !ok {
			return nil
		}

		v := strings.SplitN(raw, string(types.Separator), 3)

		if len(v) != 2 {
			return fmt.Errorf("failed to parse %q: expected 'namespace/name' format", key)
		}

		for _, val := range v {
			if errs := validation.ValidateNamespaceName(val, false); len(errs) > 0 {
				return fmt.Errorf("failed to parse %q: %s", key, strings.Join(errs, ", "))
			}
		}

		target.Namespace = v[0]
		target.Name = v[1]

		return nil
	}
}

// Parse parses the given map using the parser functions passed in.
func Parse(data map[string]string, parsers ...ParseFunc) error {
	for _, parse := range parsers {
		if err := parse(data); err != nil {
			return err
		}
	}
	return nil
}

// CollectMapEntriesWithPrefix parses the data into the target as a map[string]string, if it exists.
// The map is represented as a list of key-value pairs with a common prefix.
func CollectMapEntriesWithPrefix(prefix string, target *map[string]string) ParseFunc {
	if target == nil {
		panic("target cannot be nil")
	}

	return func(data map[string]string) error {
		for k, v := range data {
			if strings.HasPrefix(k, prefix) && len(k) > len(prefix)+1 {
				if *target == nil {
					m := make(map[string]string, 2)
					*target = m
				}
				(*target)[k[len(prefix)+1: /* remove dot `.` */]] = v
			}
		}
		return nil
	}
}
