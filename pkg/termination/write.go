/*
Copyright 2019 The Tekton Authors

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

package termination

import (
	"bytes"
	"compress/flate"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/tektoncd/pipeline/pkg/result"
)

const (
	// MaxContainerTerminationMessageLength is the upper bound any one container may write to
	// its termination message path. Contents above this length will cause a failure.
	MaxContainerTerminationMessageLength = 1024 * 4

	// compressedPrefix is prepended to compressed termination messages so the
	// parser can distinguish compressed from plain JSON messages.
	compressedPrefix = "tknz:"
)

// WriteMessage writes the results to the termination message path.
func WriteMessage(path string, pro []result.RunResult) error {
	return writeMessage(path, pro, false)
}

// WriteCompressedMessage writes the results to the termination message path
// using flate compression and base64 encoding to fit more data in the 4KB
// Kubernetes termination message limit.
func WriteCompressedMessage(path string, pro []result.RunResult) error {
	return writeMessage(path, pro, true)
}

func writeMessage(path string, pro []result.RunResult, compress bool) error {
	// if the file at path exists, concatenate the new values otherwise create it
	fileContents, err := os.ReadFile(path)
	if err == nil {
		existing, err := parseExisting(fileContents)
		if err == nil {
			pro = append(existing, pro...)
		}
	} else if !os.IsNotExist(err) {
		return err
	}

	jsonOutput, err := json.Marshal(pro)
	if err != nil {
		return err
	}

	var output []byte
	if compress {
		compressed, err := compressMessage(jsonOutput)
		if err != nil {
			return err
		}
		output = compressed
	} else {
		output = jsonOutput
	}

	if len(output) > MaxContainerTerminationMessageLength {
		return errTooLong
	}

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err = f.Write(output); err != nil {
		return err
	}
	return f.Sync()
}

// parseExisting attempts to parse existing termination message contents,
// handling both compressed and plain JSON formats.
func parseExisting(data []byte) ([]result.RunResult, error) {
	// Try compressed format first
	if bytes.HasPrefix(data, []byte(compressedPrefix)) {
		decompressed, err := decompressMessage(data)
		if err != nil {
			return nil, err
		}
		var entries []result.RunResult
		if err := json.Unmarshal(decompressed, &entries); err != nil {
			return nil, err
		}
		return entries, nil
	}
	// Fall back to plain JSON
	var entries []result.RunResult
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

// compressMessage compresses JSON data with flate and base64-encodes it,
// prepending the compressed prefix for identification.
func compressMessage(jsonData []byte) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(compressedPrefix)

	b64Writer := base64.NewEncoder(base64.StdEncoding, &buf)
	flateWriter, err := flate.NewWriter(b64Writer, flate.BestCompression)
	if err != nil {
		return nil, err
	}
	if _, err := flateWriter.Write(jsonData); err != nil {
		return nil, err
	}
	if err := flateWriter.Close(); err != nil {
		return nil, err
	}
	if err := b64Writer.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// decompressMessage decodes and decompresses a "tknz:"-prefixed message.
func decompressMessage(data []byte) ([]byte, error) {
	encoded := data[len(compressedPrefix):]
	decoded, err := base64.StdEncoding.DecodeString(string(encoded))
	if err != nil {
		return nil, fmt.Errorf("base64 decode: %w", err)
	}
	reader := flate.NewReader(bytes.NewReader(decoded))
	defer reader.Close()
	decompressed, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("flate decompress: %w", err)
	}
	return decompressed, nil
}

// MessageLengthError indicate the length of termination message of container is beyond 4096 which is the max length read by kubenates
type MessageLengthError string

const (
	errTooLong MessageLengthError = "Termination message is above max allowed size 4096, caused by large task result."
)

func (e MessageLengthError) Error() string {
	return string(e)
}
