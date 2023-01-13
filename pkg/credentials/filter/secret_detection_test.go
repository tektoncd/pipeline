package filter_test

import (
	"os"
	"testing"

	"github.com/tektoncd/pipeline/pkg/credentials/filter"
)

func setupSymlinksSimilarToKubernetesMounts(testFolder string) (cleanup func(), err error) {
	cleanup = func() {}

	pwd, err := os.Getwd()
	if err != nil {
		return cleanup, err
	}

	absoluteTestFolder := pwd + "/" + testFolder

	err = os.Chdir(absoluteTestFolder)
	if err != nil {
		return cleanup, err
	}

	// create symlinks as it is done in kubernetes
	err = os.Symlink("..2022_06_08_07_17_26.065466550", "..data")
	if err != nil {
		return cleanup, err
	}
	cleanup1 := func() { os.Remove(absoluteTestFolder + "/..data") }
	cleanup = cleanup1
	err = os.Symlink("..data/password", "password")
	if err != nil {
		return cleanup, err
	}
	cleanup2 := func() { os.Remove(absoluteTestFolder + "/password") }
	cleanup = func() {
		cleanup1()
		cleanup2()
	}
	err = os.Symlink("..data/username", "username")
	if err != nil {
		return cleanup, err
	}
	cleanup3 := func() { os.Remove(absoluteTestFolder + "/username") }
	cleanup = func() {
		cleanup1()
		cleanup2()
		cleanup3()
	}

	err = os.Chdir(pwd)
	if err != nil {
		return cleanup, err
	}

	return cleanup, nil
}

func TestDectectionFromLocationsWithSymlinks(t *testing.T) {
	testFolder := "testdata/secret_folder_symlinks"

	cleanup, err := setupSymlinksSimilarToKubernetesMounts(testFolder)
	defer cleanup()
	if err != nil {
		t.Fatal(err)
	}

	locations := filter.SecretLocations{
		Files: []string{testFolder},
	}

	detectedSecrets, err := filter.DetectSecretsFromLocations(&locations)
	if err != nil {
		t.Fatal(err)
	}

	if len(detectedSecrets) != 2 {
		t.Fatal("Expected 2 detected secrets")
	}

	if detectedSecrets[0].Name != testFolder+"/password" && string(detectedSecrets[0].Value) != "pass" {
		t.Errorf("detected secret in env var does not match")
	}

	if detectedSecrets[1].Name != testFolder+"/username" && string(detectedSecrets[1].Value) != "user" {
		t.Errorf("detected secret in env var does not match")
	}
}

func TestDectectionFromLocations(t *testing.T) {
	secretEnvVar := "SECRET_ENV_VAR"
	secretFile := "testdata/secret_file"
	secretFolder := "testdata/secret_folder"
	locations := filter.SecretLocations{
		EnvironmentVariables: []string{secretEnvVar},
		Files:                []string{secretFile, secretFolder},
	}

	secretEnvValue := "secretInEnv"
	secretFileValue := "secretInFile"
	secretFileValue2 := "secret2InFile"

	os.Setenv(secretEnvVar, secretEnvValue)
	defer os.Unsetenv(secretEnvVar)

	detectedSecrets, err := filter.DetectSecretsFromLocations(&locations)
	if err != nil {
		t.Fatal(err)
	}

	if detectedSecrets[0].Name != secretEnvVar && string(detectedSecrets[0].Value) != secretEnvValue {
		t.Errorf("detected secret in env var does not match")
	}

	if detectedSecrets[1].Name != secretFile && string(detectedSecrets[1].Value) != secretFileValue {
		t.Errorf("detected secret in file does not match")
	}

	if detectedSecrets[2].Name != secretFolder+"/item1" && string(detectedSecrets[2].Value) != secretFileValue {
		t.Errorf("detected secret in file does not match")
	}

	if detectedSecrets[3].Name != secretFolder+"/item2" && string(detectedSecrets[3].Value) != secretFileValue {
		t.Errorf("detected secret in file does not match")
	}

	if detectedSecrets[4].Name != secretFolder+"/subpath/item3" && string(detectedSecrets[4].Value) != secretFileValue2 {
		t.Errorf("detected secret in file does not match")
	}
}
