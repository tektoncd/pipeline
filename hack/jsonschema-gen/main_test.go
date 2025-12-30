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

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestConvertOpenAPIToJSONSchema(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]interface{}
	}{
		{
			name: "basic type conversion",
			input: map[string]interface{}{
				"type":        "object",
				"description": "Test object",
			},
			expected: map[string]interface{}{
				"type":        "object",
				"description": "Test object",
			},
		},
		{
			name: "filters kubernetes extensions",
			input: map[string]interface{}{
				"type":                                 "string",
				"x-kubernetes-preserve-unknown-fields": true,
				"x-kubernetes-list-type":               "atomic",
			},
			expected: map[string]interface{}{
				"type": "string",
			},
		},
		{
			name: "handles nested properties",
			input: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":                                 "string",
						"x-kubernetes-preserve-unknown-fields": true,
					},
				},
			},
			expected: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type": "string",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := make(map[string]interface{})
			convertOpenAPIToJSONSchema(tt.input, result)

			// Compare using JSON marshaling for deep equality
			expectedJSON, _ := json.Marshal(tt.expected)
			resultJSON, _ := json.Marshal(result)

			if string(expectedJSON) != string(resultJSON) {
				t.Errorf("convertOpenAPIToJSONSchema() = %s, want %s", resultJSON, expectedJSON)
			}
		})
	}
}

func TestConvertToJSONSchema(t *testing.T) {
	input := map[string]interface{}{
		"type":        "object",
		"description": "Test Task",
		"properties": map[string]interface{}{
			"apiVersion": map[string]interface{}{
				"type": "string",
			},
			"kind": map[string]interface{}{
				"type": "string",
			},
		},
	}

	result := convertToJSONSchema(input, "tekton.dev", "v1", "Task")

	// Verify JSON Schema meta fields
	if result["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Errorf("Expected $schema to be set, got %v", result["$schema"])
	}

	if result["$id"] != "https://tekton.dev/schemas/v1/task.json" {
		t.Errorf("Expected $id to be set correctly, got %v", result["$id"])
	}

	if result["title"] != "Tekton Task v1" {
		t.Errorf("Expected title to be set correctly, got %v", result["title"])
	}

	// Verify properties are preserved
	props, ok := result["properties"].(map[string]interface{})
	if !ok {
		t.Error("Expected properties to be a map")
	}

	if _, ok := props["apiVersion"]; !ok {
		t.Error("Expected apiVersion property to be preserved")
	}
}

func TestProcessCRDFile(t *testing.T) {
	// Create a temporary directory for test output
	tmpDir, err := os.MkdirTemp("", "jsonschema-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a simple test CRD file
	testCRD := `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: tasks.tekton.dev
spec:
  group: tekton.dev
  names:
    kind: Task
    plural: tasks
    singular: task
  versions:
    - name: v1
      served: true
      storage: true
      schema:
        openAPIV3Schema:
          type: object
          description: Test Task
          properties:
            apiVersion:
              type: string
            kind:
              type: string
`
	crdFile := filepath.Join(tmpDir, "test-task.yaml")
	if err := os.WriteFile(crdFile, []byte(testCRD), 0644); err != nil {
		t.Fatalf("Failed to write test CRD: %v", err)
	}

	outputDir := filepath.Join(tmpDir, "output")
	if err := processCRDFile(crdFile, outputDir, ""); err != nil {
		t.Fatalf("processCRDFile failed: %v", err)
	}

	// Verify output file was created
	outputFile := filepath.Join(outputDir, "v1", "task.json")
	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		t.Errorf("Expected output file %s to exist", outputFile)
	}

	// Verify output is valid JSON
	data, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal(data, &schema); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}

	// Verify schema has required fields
	if schema["$schema"] == nil {
		t.Error("Expected $schema field in output")
	}
	if schema["$id"] == nil {
		t.Error("Expected $id field in output")
	}
}
