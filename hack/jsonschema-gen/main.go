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
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// CRD represents a Kubernetes CustomResourceDefinition
type CRD struct {
	APIVersion string `yaml:"apiVersion"`
	Kind       string `yaml:"kind"`
	Metadata   struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Spec struct {
		Group    string `yaml:"group"`
		Versions []struct {
			Name    string `yaml:"name"`
			Served  bool   `yaml:"served"`
			Storage bool   `yaml:"storage"`
			Schema  struct {
				OpenAPIV3Schema map[string]interface{} `yaml:"openAPIV3Schema"`
			} `yaml:"schema"`
		} `yaml:"versions"`
		Names struct {
			Kind     string `yaml:"kind"`
			Singular string `yaml:"singular"`
			Plural   string `yaml:"plural"`
		} `yaml:"names"`
	} `yaml:"spec"`
}

// JSONSchema represents a JSON Schema document
type JSONSchema struct {
	Schema      string                 `json:"$schema"`
	ID          string                 `json:"$id"`
	Title       string                 `json:"title"`
	Description string                 `json:"description,omitempty"`
	Type        interface{}            `json:"type,omitempty"`
	Properties  map[string]interface{} `json:"properties,omitempty"`
	Required    []string               `json:"required,omitempty"`
	// Additional fields from OpenAPIV3Schema
	AdditionalProperties interface{} `json:"additionalProperties,omitempty"`
	Items                interface{} `json:"items,omitempty"`
	Enum                 []string    `json:"enum,omitempty"`
	Default              interface{} `json:"default,omitempty"`
	Format               string      `json:"format,omitempty"`
	Pattern              string      `json:"pattern,omitempty"`
	Minimum              interface{} `json:"minimum,omitempty"`
	Maximum              interface{} `json:"maximum,omitempty"`
	MinLength            interface{} `json:"minLength,omitempty"`
	MaxLength            interface{} `json:"maxLength,omitempty"`
	MinItems             interface{} `json:"minItems,omitempty"`
	MaxItems             interface{} `json:"maxItems,omitempty"`
	UniqueItems          bool        `json:"uniqueItems,omitempty"`
	OneOf                interface{} `json:"oneOf,omitempty"`
	AnyOf                interface{} `json:"anyOf,omitempty"`
	AllOf                interface{} `json:"allOf,omitempty"`
	Not                  interface{} `json:"not,omitempty"`
}

func main() {
	crdDir := flag.String("crd-dir", "config/300-crds", "Directory containing CRD YAML files")
	outputDir := flag.String("output-dir", "schema", "Output directory for JSON Schema files")
	apiVersion := flag.String("api-version", "", "API version to generate (empty for all)")
	flag.Parse()

	// Create output directory if it doesn't exist
	if err := os.MkdirAll(*outputDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating output directory: %v\n", err)
		os.Exit(1)
	}

	// Find all CRD files
	files, err := filepath.Glob(filepath.Join(*crdDir, "*.yaml"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error finding CRD files: %v\n", err)
		os.Exit(1)
	}

	for _, file := range files {
		if err := processCRDFile(file, *outputDir, *apiVersion); err != nil {
			fmt.Fprintf(os.Stderr, "Error processing %s: %v\n", file, err)
			os.Exit(1)
		}
	}

	fmt.Println("JSON Schema generation complete.")
}

func processCRDFile(filePath, outputDir, targetAPIVersion string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	var crd CRD
	if err := yaml.Unmarshal(data, &crd); err != nil {
		return fmt.Errorf("parsing YAML: %w", err)
	}

	// Skip non-CRD files
	if crd.Kind != "CustomResourceDefinition" {
		return nil
	}

	group := crd.Spec.Group
	kind := crd.Spec.Names.Kind

	for _, version := range crd.Spec.Versions {
		// Skip if specific version requested and this isn't it
		if targetAPIVersion != "" && version.Name != targetAPIVersion {
			continue
		}

		// Skip versions without schema
		if version.Schema.OpenAPIV3Schema == nil {
			continue
		}

		// Create version-specific output directory
		versionDir := filepath.Join(outputDir, version.Name)
		if err := os.MkdirAll(versionDir, 0755); err != nil {
			return fmt.Errorf("creating version directory: %w", err)
		}

		schema := convertToJSONSchema(version.Schema.OpenAPIV3Schema, group, version.Name, kind)

		// Write JSON Schema file
		outputFile := filepath.Join(versionDir, strings.ToLower(kind)+".json")
		jsonData, err := json.MarshalIndent(schema, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON: %w", err)
		}

		if err := os.WriteFile(outputFile, jsonData, 0644); err != nil {
			return fmt.Errorf("writing file: %w", err)
		}

		fmt.Printf("Generated %s\n", outputFile)
	}

	return nil
}

func convertToJSONSchema(openAPISchema map[string]interface{}, group, version, kind string) map[string]interface{} {
	schema := make(map[string]interface{})

	// Add JSON Schema meta fields
	schema["$schema"] = "https://json-schema.org/draft/2020-12/schema"
	schema["$id"] = fmt.Sprintf("https://tekton.dev/schemas/%s/%s.json", version, strings.ToLower(kind))
	schema["title"] = fmt.Sprintf("Tekton %s %s", kind, version)

	// Copy and convert OpenAPI schema properties
	convertOpenAPIToJSONSchema(openAPISchema, schema)

	return schema
}

func convertOpenAPIToJSONSchema(src map[string]interface{}, dst map[string]interface{}) {
	for key, value := range src {
		switch key {
		// Skip OpenAPI-specific fields that don't apply to JSON Schema
		case "x-kubernetes-preserve-unknown-fields",
			"x-kubernetes-list-type",
			"x-kubernetes-list-map-keys",
			"x-kubernetes-map-type",
			"x-kubernetes-patch-strategy",
			"x-kubernetes-patch-merge-key",
			"x-kubernetes-embedded-resource",
			"x-kubernetes-int-or-string",
			"x-kubernetes-group-version-kind",
			"x-kubernetes-validations":
			// These are Kubernetes-specific extensions, skip them
			continue
		default:
			// For nested objects, recursively convert
			if mapValue, ok := value.(map[string]interface{}); ok {
				newMap := make(map[string]interface{})
				convertOpenAPIToJSONSchema(mapValue, newMap)
				dst[key] = newMap
			} else if sliceValue, ok := value.([]interface{}); ok {
				// Handle arrays of objects
				newSlice := make([]interface{}, len(sliceValue))
				for i, item := range sliceValue {
					if itemMap, ok := item.(map[string]interface{}); ok {
						newItem := make(map[string]interface{})
						convertOpenAPIToJSONSchema(itemMap, newItem)
						newSlice[i] = newItem
					} else {
						newSlice[i] = item
					}
				}
				dst[key] = newSlice
			} else {
				dst[key] = value
			}
		}
	}
}
