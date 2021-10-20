/*
Copyright 2020 The Tekton Authors

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
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
)

// DecodeScriptCommand is the command name for decoding scripts.
const DecodeScriptCommand = "decode-script"

// decodeScript rewrites a script file from base64 back into its original content from
// the Step definition.
func decodeScript(scriptPath string) error {
	decodedBytes, permissions, err := decodeScriptFromFile(scriptPath)
	if err != nil {
		return fmt.Errorf("error decoding script file %q: %w", scriptPath, err)
	}
	err = ioutil.WriteFile(scriptPath, decodedBytes, permissions)
	if err != nil {
		return fmt.Errorf("error writing decoded script file %q: %w", scriptPath, err)
	}
	return nil
}

// decodeScriptFromFile reads the script at scriptPath, decodes it from
// base64, and returns the decoded bytes w/ the permissions to use when re-writing
// or an error.
func decodeScriptFromFile(scriptPath string) ([]byte, os.FileMode, error) {
	scriptFile, err := os.Open(scriptPath)
	if err != nil {
		return nil, 0, fmt.Errorf("error reading from script file %q: %w", scriptPath, err)
	}
	defer scriptFile.Close()

	encoded := bytes.NewBuffer(nil)
	if _, err = io.Copy(encoded, scriptFile); err != nil {
		return nil, 0, fmt.Errorf("error reading from script file %q: %w", scriptPath, err)
	}

	fileInfo, err := scriptFile.Stat()
	if err != nil {
		return nil, 0, fmt.Errorf("error statting script file %q: %w", scriptPath, err)
	}
	perms := fileInfo.Mode().Perm()

	decoded := make([]byte, base64.StdEncoding.DecodedLen(encoded.Len()))
	n, err := base64.StdEncoding.Decode(decoded, encoded.Bytes())
	if err != nil {
		return nil, 0, fmt.Errorf("error decoding script file %q: %w", scriptPath, err)
	}
	return decoded[0:n], perms, nil
}
