//go:build !disable_tls

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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/tektoncd/pipeline/pkg/result"
	"go.uber.org/zap"
)

// ParseMessage parses a termination message as results.
//
// If more than one item has the same key, only the latest is returned. Items
// are sorted by their key.
//
// Automatically detects and decompresses messages that were compressed with
// WriteCompressedMessage (identified by the "tknz:" prefix).
func ParseMessage(logger *zap.SugaredLogger, msg string) ([]result.RunResult, error) {
	if msg == "" {
		return nil, nil
	}

	// Auto-detect compressed messages
	jsonMsg := msg
	if strings.HasPrefix(msg, compressedPrefix) {
		decompressed, err := decompressMessage([]byte(msg))
		if err != nil {
			return nil, fmt.Errorf("decompressing termination message: %w", err)
		}
		jsonMsg = string(decompressed)
	}

	var r []result.RunResult
	if err := json.Unmarshal([]byte(jsonMsg), &r); err != nil {
		// Truncate the message in logs to avoid enormous entries from decompressed payloads.
		truncated := jsonMsg
		if len(truncated) > 256 {
			truncated = truncated[:256] + "...(truncated)"
		}
		return nil, fmt.Errorf("parsing message json: %w, msg: %s", err, truncated)
	}

	writeIndex := 0
	for _, rr := range r {
		if rr != (result.RunResult{}) {
			// Erase incorrect result
			r[writeIndex] = rr
			writeIndex++
		} else {
			logger.Errorf("termination message contains non taskrun or pipelineresource result keys")
		}
	}
	r = r[:writeIndex]

	// Remove duplicates (last one wins) and sort by key.
	m := map[string]result.RunResult{}
	for _, rr := range r {
		m[rr.Key] = rr
	}
	r2 := make([]result.RunResult, 0, len(m))
	for _, v := range m {
		r2 = append(r2, v)
	}
	sort.Slice(r2, func(i, j int) bool { return r2[i].Key < r2[j].Key })

	return r2, nil
}
