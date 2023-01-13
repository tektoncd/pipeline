package filter

import (
	"encoding/json"
	"fmt"
	"os"
)

// SecretLocations describes the locations of all secrets in a pipeline pod.
// The entrypoint uses it to find all secrets that he should filter/redact from
// the output logs.
type SecretLocations struct {
	// Names of all environment variables that contain secrets.
	EnvironmentVariables []string `json:"environmentVariables"`
	// Absolute paths to all files that contain mounted secret values.
	Files []string `json:"files"`
}

// WriteSecretLocationsToFile writes a json encoded secret locations object into the default file.
// Mainly used for testing.
func WriteSecretLocationsToFile(secretLocations *SecretLocations) (func(), error) {
	cleanup := func() {}
	bytes, err := json.Marshal(secretLocations)
	if err != nil {
		return cleanup, err
	}

	err = os.MkdirAll(downwardMountPath, 0777)
	if err != nil {
		return cleanup, err
	}

	err = os.WriteFile(downwardMountPath+downwardMountSecretLocationsFile, bytes, 0600)
	if err != nil {
		return cleanup, err
	}

	cleanup = func() {
		os.Remove(downwardMountPath + downwardMountSecretLocationsFile)
	}

	return cleanup, nil
}

// NewSecretLocationsFromFile will retrieve the locations of mounted secrets from the file /tekton/downward/secret-locations.json.
// This is executed in the entrypoint in each step to find the secrets that need to be redacted.
func NewSecretLocationsFromFile() (*SecretLocations, error) {
	secretLocationsBytes, err := os.ReadFile(downwardMountPath + downwardMountSecretLocationsFile)
	if err != nil {
		return nil, err
	}
	return ParseSecretLocations(string(secretLocationsBytes))
}

// ParseSecretLocations takes a json string and tries to parse it into a SecretLocations struct.
func ParseSecretLocations(jsonValue string) (*SecretLocations, error) {
	secretLocations := SecretLocations{}
	err := json.Unmarshal([]byte(jsonValue), &secretLocations)
	if err != nil {
		return nil, fmt.Errorf("could not decode secret locations from json: %v\n%s", jsonValue, err)
	}

	return &secretLocations, nil
}
