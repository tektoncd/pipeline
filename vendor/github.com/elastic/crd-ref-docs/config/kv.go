// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package config

import (
	"fmt"
	"strings"
)

type KeyValue struct {
	Key   string
	Value string
}
type KeyValueFlags []KeyValue

// KeyValue is an implementation of the flag.Value interface
func (kvs *KeyValueFlags) String() string {
	return fmt.Sprintf("%v", *kvs)
}

// Type implements pflag.Value
func (kvs *KeyValueFlags) Type() string {
	return "key value"
}

func (kvs *KeyValueFlags) AsMap() map[string]string {
	if kvs == nil {
		return nil
	}
	m := make(map[string]string, len(*kvs))
	for _, kv := range *kvs {
		m[kv.Key] = kv.Value
	}
	return m
}

// Set is an implementation of the flag.Value interface
func (kvs *KeyValueFlags) Set(value string) error {
	if value == "" {
		return nil
	}
	parts := strings.SplitN(value, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid key-value pair: %s, expected format key=value", value)
	}
	key := strings.TrimSpace(parts[0])
	value = strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return fmt.Errorf("key and value must not be empty: %s", value)
	}
	kv := KeyValue{Key: key, Value: value}
	*kvs = append(*kvs, kv)
	return nil
}
