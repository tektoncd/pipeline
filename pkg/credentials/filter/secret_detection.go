package filter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DetectedSecret represents a secret that was detected in the entrypoint.
type DetectedSecret struct {
	Name  string
	Value []byte
}

// DetectSecretsFromLocations detects secrets based on the locations where they are provided
// in the pipeline run pod. The locations are provided by the pipeline controller.
// This is executed in the entrypoint runner to actually load the secret values that
// are attached to the pod.
func DetectSecretsFromLocations(locations *SecretLocations) ([]*DetectedSecret, error) {
	detectSecrets := make([]*DetectedSecret, 0)

	for _, envVar := range locations.EnvironmentVariables {
		detectSecrets = append(detectSecrets, &DetectedSecret{
			Name:  envVar,
			Value: []byte(os.Getenv(envVar)),
		})
	}

	for _, secretFile := range locations.Files {
		addSecretFromFile := func(path string) error {
			content, err := os.ReadFile(path)
			if err != nil {
				fileInfo := "no file info"
				info, err2 := os.Stat(path)
				if err2 == nil {
					fileInfo = fmt.Sprintf("name: %s, isDir: %t", info.Name(), info.IsDir())
				}

				return fmt.Errorf("could not read secret file %s (%s) to redact its value from log output: %w", path, fileInfo, err)
			}
			detectSecrets = append(detectSecrets, &DetectedSecret{
				Name:  path,
				Value: content,
			})
			return nil
		}

		info, err := os.Stat(secretFile)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			err = filepath.WalkDir(secretFile, func(path string, entry os.DirEntry, err error) error {
				resolvedPath, err := filepath.EvalSymlinks(path)
				if err != nil {
					return err
				}
				resolvedInfo, err := os.Stat(resolvedPath)
				if err != nil {
					return err
				}

				if resolvedInfo.IsDir() {
					// we want to traverse all paths
					return nil
				}

				// two dots are used to hide symlinked folders in kubernetes
				// we skip all files with a path in those folders
				// else we get duplicates because they also have a symlink representation
				if strings.Contains(path, "..") {
					return filepath.SkipDir
				}

				return addSecretFromFile(path)
			})
			if err != nil {
				return nil, err
			}
		} else {
			err = addSecretFromFile(secretFile)
			if err != nil {
				return nil, err
			}
		}
	}

	return detectSecrets, nil
}
