/*
Copyright 2021 The Knative Authors

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

package json

import (
	"bytes"
	"encoding/json"
	"io"
)

var (
	emptyMeta  = []byte(`:{}`)
	metaPrefix = []byte(`{"metadata"`)
	metaSuffix = []byte(`}`)
)

var (
	// Unmarshal is an alias for json.Unmarshal
	Unmarshal = json.Unmarshal

	//Marshal is an alias for json.Marshal
	Marshal = json.Marshal
)

// Decode will parse the json byte array to the target object. When
// unknown fields are _not_ allowed we still accept unknown
// fields in the Object's metadata
//
// See https://github.com/knative/serving/issues/11448 for details
func Decode(bites []byte, target interface{}, disallowUnknownFields bool) error {
	if !disallowUnknownFields {
		return json.Unmarshal(bites, target)
	}

	// If we don't allow unknown fields we skip validating fields in the metadata
	// block since that is opaque to us and validated by the API server
	start, end, err := findMetadataOffsets(bites)
	if err != nil {
		return err
	} else if start == -1 || end == -1 {
		// If for some reason the json does not have metadata continue with normal parsing
		dec := json.NewDecoder(bytes.NewReader(bites))
		dec.DisallowUnknownFields()
		return dec.Decode(target)
	}

	before := bites[:start]
	metadata := bites[start:end]
	after := bites[end:]

	// Parse everything but skip metadata
	dec := json.NewDecoder(io.MultiReader(
		bytes.NewReader(before),
		bytes.NewReader(emptyMeta),
		bytes.NewReader(after),
	))

	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		return err
	}

	// Now we parse just the metadata
	dec = json.NewDecoder(io.MultiReader(
		bytes.NewReader(metaPrefix),
		bytes.NewReader(metadata),
		bytes.NewReader(metaSuffix),
	))

	return dec.Decode(target)
}

func findMetadataOffsets(bites []byte) (start, end int64, err error) {
	start, end = -1, -1
	level := 0

	var (
		dec = json.NewDecoder(bytes.NewReader(bites))
		t   json.Token
	)

	for {
		t, err = dec.Token()
		if err == io.EOF { //nolint
			break
		}
		if err != nil {
			return
		}

		switch v := t.(type) {
		case json.Delim:
			if v == '{' {
				level++
			} else if v == '}' {
				level--
			}
		case string:
			if v == "metadata" && level == 1 {
				start = dec.InputOffset()
				x := struct{}{}
				if err = dec.Decode(&x); err != nil {
					return -1, -1, err
				}
				end = dec.InputOffset()

				// we exit early to stop processing the rest of the object
				return
			}
		}
	}
	return -1, -1, nil
}
