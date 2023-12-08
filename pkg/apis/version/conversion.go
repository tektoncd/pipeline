/*
Copyright 2022 The Tekton Authors

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

package version

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SerializeToMetadata serializes the input field and adds it as an annotation to
// the metadata under the input key.
func SerializeToMetadata(meta *metav1.ObjectMeta, field interface{}, key string) error {
	data, err := json.Marshal(field)
	if err != nil {
		return fmt.Errorf("error serializing field: %s", err)
	}

	// The data coming from TaskRun and PipelineRun workloads can be really huge
	// and can result in inflated annotations which exceed the maximum allowed size
	// for annotations of 256 kB, so we will be compressing and encoding the data.
	encodedData, err := CompressAndEncode(data)
	if err != nil {
		return err
	}

	// Encoding to normalize the data for annotations
	if meta.Annotations == nil {
		meta.Annotations = make(map[string]string)
	}
	meta.Annotations[key] = encodedData

	// While we have compressed the payload, we can still not guarantee that the
	// maximum allowed annotation size (256 kB) is not exceeded :(
	if err := validation.ValidateAnnotationsSize(meta.Annotations); err != nil {
		return err
	}
	return nil
}

// DeserializeFromMetadata takes the value of the input key from the metadata's annotations,
// deserializes it into "to", and removes the key from the metadata's annotations.
// Returns nil if the key is not present in the annotations.
func DeserializeFromMetadata(meta *metav1.ObjectMeta, to interface{}, key string) error {
	if meta.Annotations == nil {
		return nil
	}
	if str, ok := meta.Annotations[key]; ok {
		// This data needs to be first decoded and then decompressed.
		data, err := DecodeAndDecompress(str)
		if err != nil {
			return err
		}

		if err := json.Unmarshal(data, to); err != nil {
			return fmt.Errorf("error deserializing key %s from metadata: %w", key, err)
		}
		delete(meta.Annotations, key)
		if len(meta.Annotations) == 0 {
			meta.Annotations = nil
		}
	}
	return nil
}

func CompressAndEncode(data []byte) (string, error) {
	// Compressing input data
	var compressed bytes.Buffer
	gz := gzip.NewWriter(&compressed)
	if _, err := gz.Write(data); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}

	// Encoding compressed data
	return base64.StdEncoding.EncodeToString(compressed.Bytes()), nil
}

func DecodeAndDecompress(data string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, err
	}
	gz, err := gzip.NewReader(bytes.NewReader(decoded))
	if err != nil {
		return nil, fmt.Errorf("error decoding string from encoded marshalled bytes %w", err)
	}
	decompressedData, err := io.ReadAll(gz)
	if err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}
	return decompressedData, nil
}
