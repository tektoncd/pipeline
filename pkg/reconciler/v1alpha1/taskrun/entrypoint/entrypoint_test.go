package entrypoint_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strings"
	"testing"

	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/knative/build/pkg/apis/build/v1alpha1"
	corev1 "k8s.io/api/core/v1"

	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/config"
	"github.com/knative/build-pipeline/pkg/reconciler/v1alpha1/taskrun/entrypoint"
)

func TestAddEntrypoint(t *testing.T) {
	inputs := []corev1.Container{
		{
			Image: "image",
		},
		{
			Image: "image:tag",
			Args:  []string{"abcd"},
		},
		{
			Image:   "my.registry.svc/image:tag",
			Command: []string{"abcd"},
			Args:    []string{"efgh"},
		},
	}
	// The first test case showcases the downloading of the entrypoint for the
	// image. The second test shows downloading the image as well as the args
	// being passed in. The third command shows a set Command overriding the
	// remote one.
	envVarStrings := []string{
		`{"args":null,"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
		`{"args":["abcd"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
		`{"args":["abcd","efgh"],"process_log":"/tools/process-log.txt","marker_file":"/tools/marker-file.txt"}`,
	}
	err := entrypoint.RedirectSteps(inputs)
	if err != nil {
		t.Errorf("failed to get resources: %v", err)
	}
	for i, input := range inputs {
		if len(input.Command) == 0 || input.Command[0] != entrypoint.BinaryLocation {
			t.Errorf("command incorrectly set: %q", input.Command)
		}
		if len(input.Args) > 0 {
			t.Errorf("containers should have no args")
		}
		if len(input.Env) == 0 {
			t.Error("there should be atleast one envvar")
		}
		for _, e := range input.Env {
			if e.Name == entrypoint.JSONConfigEnvVar && e.Value != envVarStrings[i] {
				t.Errorf("envvar \n%s\n does not match \n%s", e.Value, envVarStrings[i])
			}
		}
		found := false
		for _, vm := range input.VolumeMounts {
			if vm.Name == entrypoint.MountName {
				found = true
				break
			}
		}
		if !found {
			t.Error("could not find tools volume mount")
		}
	}
}

type image struct {
	config *v1.ConfigFile
}

// RawConfigFile implements partial.UncompressedImageCore
func (i *image) RawConfigFile() ([]byte, error) {
	return partial.RawConfigFile(i)
}

// ConfigFile implements v1.Image
func (i *image) ConfigFile() (*v1.ConfigFile, error) {
	return i.config, nil
}

// MediaType implements partial.UncompressedImageCore
func (i *image) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}

// LayerByDiffID implements partial.UncompressedImageCore
func (i *image) LayerByDiffID(diffID v1.Hash) (partial.UncompressedLayer, error) {
	return nil, fmt.Errorf("unknown diff_id: %v", diffID)
}

func mustRawManifest(t *testing.T, img v1.Image) []byte {
	m, err := img.RawManifest()
	if err != nil {
		t.Fatalf("RawManifest() = %v", err)
	}
	return m
}

func mustRawConfigFile(t *testing.T, img v1.Image) []byte {
	c, err := img.RawConfigFile()
	if err != nil {
		t.Fatalf("RawConfigFile() = %v", err)
	}
	return c
}

func getImage(t *testing.T, cfg *v1.ConfigFile) v1.Image {
	rnd, err := partial.UncompressedToImage(&image{
		config: cfg,
	})
	if err != nil {
		t.Fatalf("getImage() = %v", err)
	}
	return rnd
}

func mustConfigName(t *testing.T, img v1.Image) v1.Hash {
	h, err := img.ConfigName()
	if err != nil {
		t.Fatalf("ConfigName() = %v", err)
	}
	return h
}

func TestGetRemoteEntrypoint(t *testing.T) {
	expectedEntrypoint := []string{"/bin/expected", "entrypoint"}
	img := getImage(t, &v1.ConfigFile{
		ContainerConfig: v1.Config{
			Entrypoint: expectedEntrypoint,
		},
	})
	expectedRepo := "image"
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))
	manifestPath := fmt.Sprintf("/v2/%s/manifests/latest", expectedRepo)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawConfigFile(t, img))
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			w.Write(mustRawManifest(t, img))
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	image := path.Join(strings.TrimPrefix(server.URL, "http://"), "image:latest")
	ep, err := entrypoint.GetRemoteEntrypoint(entrypoint.NewCache(), image)
	if err != nil {
		t.Errorf("couldn't get entrypoint remote: %v", err)
	}
	if !reflect.DeepEqual(ep, expectedEntrypoint) {
		t.Errorf("entrypoints do not match: %s should be %s", ep[0], expectedEntrypoint)
	}
}

func TestAddCopyStep(t *testing.T) {
	cfg := &config.Config{
		Entrypoint: &config.Entrypoint{
			Image: config.DefaultEntrypointImage,
		},
	}
	ctx := config.ToContext(context.Background(), cfg)

	bs := &v1alpha1.BuildSpec{
		Steps: []corev1.Container{
			{
				Name: "test",
			},
			{
				Name: "test",
			},
		},
	}

	expectedSteps := len(bs.Steps) + 1
	entrypoint.AddCopyStep(ctx, bs)
	if len(bs.Steps) != 3 {
		t.Errorf("BuildSpec has the wrong step count: %d should be %d", len(bs.Steps), expectedSteps)
	}
	if bs.Steps[0].Name != entrypoint.InitContainerName {
		t.Errorf("entrypoint is incorrect: %s should be %s", bs.Steps[0].Name, entrypoint.InitContainerName)
	}
}
