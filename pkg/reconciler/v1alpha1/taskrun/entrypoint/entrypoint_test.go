package entrypoint

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"golang.org/x/xerrors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
)

const (
	exceedCacheSize = 10
)

func TestRewriteSteps(t *testing.T) {
	inputs := []corev1.Container{
		{
			Image:   "image",
			Command: []string{"abcd"},
		},
		{
			Image:   "my.registry.svc/image:tag",
			Command: []string{"abcd"},
			Args:    []string{"efgh"},
		},
	}
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "taskRun",
		},
		Spec: v1alpha1.TaskRunSpec{
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []corev1.Container{{
					Image:   "ubuntu",
					Command: []string{"echo"},
					Args:    []string{"hello"},
				}},
			},
		},
	}
	observer, _ := observer.New(zap.InfoLevel)
	entrypointCache, _ := NewCache()
	c := fakekubeclientset.NewSimpleClientset()
	err := RedirectSteps(entrypointCache, inputs, c, taskRun, zap.New(observer).Sugar())
	if err != nil {
		t.Errorf("failed to get resources: %v", err)
	}
	for _, input := range inputs {
		if len(input.Command) == 0 || input.Command[0] != BinaryLocation {
			t.Errorf("command incorrectly set: %q", input.Command)
		}
		found := false
		for _, vm := range input.VolumeMounts {
			if vm.Name == MountName {
				found = true
				break
			}
		}
		if !found {
			t.Error("could not find tools volume mount")
		}
	}
}

func TestGetArgs(t *testing.T) {
	// first step
	// multiple commands
	// no args
	for _, c := range []struct {
		desc         string
		stepNum      int
		commands     []string
		args         []string
		expectedArgs []string
	}{{
		desc:     "Args for first step",
		stepNum:  0,
		commands: []string{"echo"},
		args:     []string{"hello", "world"},
		expectedArgs: []string{
			"-wait_file", "/builder/downward/ready",
			"-post_file", "/builder/tools/0",
			"-wait_file_content",
			"-entrypoint", "echo",
			"--",
			"hello", "world",
		},
	}, {
		desc:     "Multiple commands",
		stepNum:  4,
		commands: []string{"echo", "hello"},
		args:     []string{"world"},
		expectedArgs: []string{
			"-wait_file", "/builder/tools/3",
			"-post_file", "/builder/tools/4",
			"-entrypoint", "echo",
			"--",
			"hello", "world",
		},
	}, {
		desc:     "No args",
		stepNum:  4,
		commands: []string{"ls"},
		args:     []string{},
		expectedArgs: []string{
			"-wait_file", "/builder/tools/3",
			"-post_file", "/builder/tools/4",
			"-entrypoint", "ls",
			"--",
		},
	}} {
		t.Run(c.desc, func(t *testing.T) {
			a := GetArgs(c.stepNum, c.commands, c.args)
			if d := cmp.Diff(a, c.expectedArgs); d != "" {
				t.Errorf("Didn't get expected arguments, difference: %s", d)
			}
		})
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
	return nil, xerrors.Errorf("unknown diff_id: %v", diffID)
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

func getDigestAsString(image v1.Image) string {
	digestHash, _ := image.Digest()
	return digestHash.String()
}

func TestGetRemoteEntrypoint(t *testing.T) {
	expectedEntrypoint := []string{"/bin/expected", "entrypoint"}
	img := getImage(t, &v1.ConfigFile{
		Config: v1.Config{
			Entrypoint: expectedEntrypoint,
		},
	})
	expectedRepo := "image"
	digetsSha := getDigestAsString(img)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))
	manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, digetsSha)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			if _, err := w.Write(mustRawConfigFile(t, img)); err != nil {
				t.Fatal(err)
			}
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			if _, err := w.Write(mustRawManifest(t, img)); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	image := path.Join(strings.TrimPrefix(server.URL, "http://"), expectedRepo)
	finalDigest := image + "@" + digetsSha

	entrypointCache, err := NewCache()
	if err != nil {
		t.Fatalf("couldn't create new entrypoint cache: %v", err)
	}
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "taskRun",
		},
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "default",
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []corev1.Container{{
					Image:   "ubuntu",
					Command: []string{"echo"},
					Args:    []string{"hello"},
				}},
			},
		},
	}
	c := fakekubeclientset.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "default",
			Namespace: "foo",
		},
	})
	ep, err := GetRemoteEntrypoint(entrypointCache, finalDigest, c, taskRun)
	if err != nil {
		t.Errorf("couldn't get entrypoint remote: %v", err)
	}
	if !reflect.DeepEqual(ep, expectedEntrypoint) {
		t.Errorf("entrypoints do not match: %s should be %s", ep[0], expectedEntrypoint)
	}
}

func TestGetRemoteEntrypointWithNonDefaultSA(t *testing.T) {
	expectedEntrypoint := []string{"/bin/expected", "entrypoint"}
	img := getImage(t, &v1.ConfigFile{
		Config: v1.Config{
			Entrypoint: expectedEntrypoint,
		},
	})
	expectedRepo := "image"
	digetsSha := getDigestAsString(img)
	configPath := fmt.Sprintf("/v2/%s/blobs/%s", expectedRepo, mustConfigName(t, img))
	manifestPath := fmt.Sprintf("/v2/%s/manifests/%s", expectedRepo, digetsSha)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/":
			w.WriteHeader(http.StatusOK)
		case configPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			if _, err := w.Write(mustRawConfigFile(t, img)); err != nil {
				t.Fatal(err)
			}
		case manifestPath:
			if r.Method != http.MethodGet {
				t.Errorf("Method; got %v, want %v", r.Method, http.MethodGet)
			}
			if _, err := w.Write(mustRawManifest(t, img)); err != nil {
				t.Fatal(err)
			}
		default:
			t.Fatalf("Unexpected path: %v", r.URL.Path)
		}
	}))
	defer server.Close()
	image := path.Join(strings.TrimPrefix(server.URL, "http://"), expectedRepo)
	finalDigest := image + "@" + digetsSha

	entrypointCache, err := NewCache()
	if err != nil {
		t.Fatalf("couldn't create new entrypoint cache: %v", err)
	}
	taskRun := &v1alpha1.TaskRun{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "foo",
			Name:      "taskRun",
		},
		Spec: v1alpha1.TaskRunSpec{
			ServiceAccount: "some-other-sa",
			TaskSpec: &v1alpha1.TaskSpec{
				Steps: []corev1.Container{{
					Image:   "ubuntu",
					Command: []string{"echo"},
					Args:    []string{"hello"},
				}},
			},
		},
	}
	c := fakekubeclientset.NewSimpleClientset(&corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-other-sa",
			Namespace: "foo",
		},
	})
	ep, err := GetRemoteEntrypoint(entrypointCache, finalDigest, c, taskRun)
	if err != nil {
		t.Errorf("couldn't get entrypoint remote: %v", err)
	}
	if !reflect.DeepEqual(ep, expectedEntrypoint) {
		t.Errorf("entrypoints do not match: %s should be %s", ep[0], expectedEntrypoint)
	}
}

func TestEntrypointCacheLRU(t *testing.T) {
	entrypoint := []string{"/bin/expected", "entrypoint"}
	entrypointCache, err := NewCache()
	if err != nil {
		t.Fatalf("couldn't create new entrypoint cache: %v", err)
	}

	for i := 0; i < cacheSize+exceedCacheSize; i++ {
		image := fmt.Sprintf("image%d:latest", i)
		entrypointCache.set(image, entrypoint)
	}
	for i := 0; i < exceedCacheSize; i++ {
		image := fmt.Sprintf("image%d:latest", i)
		if _, ok := entrypointCache.get(image); ok {
			t.Errorf("entrypoint of image %s should be expired", image)
		}
	}
	for i := exceedCacheSize; i < cacheSize+exceedCacheSize; i++ {
		image := fmt.Sprintf("image%d:latest", i)
		if _, ok := entrypointCache.get(image); !ok {
			t.Errorf("entrypoint of image %s shouldn't be expired", image)
		}
	}
}

func TestAddCopyStep(t *testing.T) {
	ts := &v1alpha1.TaskSpec{
		Steps: []corev1.Container{{
			Name: "test",
		}, {
			Name: "test",
		}},
	}

	expectedSteps := len(ts.Steps) + 1
	AddCopyStep(ts)
	if len(ts.Steps) != 3 {
		t.Errorf("BuildSpec has the wrong step count: %d should be %d", len(ts.Steps), expectedSteps)
	}
	if ts.Steps[0].Name != InitContainerName {
		t.Errorf("entrypoint is incorrect: %s should be %s", ts.Steps[0].Name, InitContainerName)
	}
}
