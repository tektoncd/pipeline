package test

import (
	"fmt"
	"strings"

	"github.com/ghodss/yaml"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/tektoncd/pipeline/pkg/apis/pipeline/v1alpha1"
)

// CreateTaskImage will push a new OCI image artifact with the provided task as a layer and return the full image
// reference with a digest to fetch the image.
func CreateTaskImage(registryHost string, task v1alpha1.TaskInterface) (string, error) {
	imgRef, err := name.ParseReference(fmt.Sprintf("%s/taskimage/%s", registryHost, task.TaskMetadata().Name))
	if err != nil {
		return "", fmt.Errorf("undexpected error producing image reference %w", err)
	}

	raw, err := yaml.Marshal(task)
	if err != nil {
		return "", fmt.Errorf("invalid sample task def %w", err)
	}

	layer, err := tarball.LayerFromReader(strings.NewReader(string(raw)))
	if err != nil {
		return "", fmt.Errorf("unexpected error adding task layer to image %w", err)
	}

	img, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			"org.opencontainers.image.title": fmt.Sprintf("task/%s", task.TaskMetadata().Name),
		},
	})
	if err != nil {
		return "", fmt.Errorf("could not add layer to image %w", err)
	}

	if err := remoteimg.Write(imgRef, img); err != nil {
		return "", fmt.Errorf("could not push example image to registry")
	}

	digest, err := img.Digest()
	if err != nil {
		return "", fmt.Errorf("could not read image digest: %w", err)
	}

	return imgRef.Context().Digest(digest.String()).String(), nil
}
