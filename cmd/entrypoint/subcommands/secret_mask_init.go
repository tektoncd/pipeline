/*
Copyright 2025 The Tekton Authors

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

package subcommands

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// SecretMaskInitCommand is the name of the secret mask initialization command
const SecretMaskInitCommand = "secret-mask-init"

// SecretMaskDataEnvVar is the env var containing base64(gzip(secret mask data)).
const SecretMaskDataEnvVar = "TEKTON_SECRET_MASK_DATA" //nolint:gosec // G101: not a hardcoded credential, just a variable name

// secretMaskInit decodes and writes the secret mask data to the specified file path.
func secretMaskInit(filePath string) error {
	data := os.Getenv(SecretMaskDataEnvVar)
	if data == "" {
		return fmt.Errorf("environment variable %s is not set", SecretMaskDataEnvVar)
	}

	compressedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return fmt.Errorf("decode %s: %w", SecretMaskDataEnvVar, err)
	}
	gzr, err := gzip.NewReader(bytes.NewReader(compressedData))
	if err != nil {
		return fmt.Errorf("decompress %s: %w", SecretMaskDataEnvVar, err)
	}
	decodedData, readErr := io.ReadAll(gzr)
	closeErr := gzr.Close()
	if readErr != nil {
		return fmt.Errorf("read decompressed %s: %w", SecretMaskDataEnvVar, readErr)
	}
	if closeErr != nil {
		return fmt.Errorf("close decompressed %s: %w", SecretMaskDataEnvVar, closeErr)
	}

	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		return err
	}
	return os.WriteFile(filePath, decodedData, 0444) //nolint:gosec // G306: intentionally world-readable, contains non-sensitive mask patterns
}
