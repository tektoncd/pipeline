package test

import (
	"fmt"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	remoteimg "github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
)

// CreateImage will push a new OCI image artifact with the provided raw data object as a layer and return the full image
// reference with a digest to fetch the image. Key must be specified as [lowercase kind]/[object name]. The image ref
// with a digest is returned.
func CreateImage(registryHost, key, data string) (string, error) {
	imgRef, err := name.ParseReference(fmt.Sprintf("%s/%s", registryHost, key))
	if err != nil {
		return "", fmt.Errorf("undexpected error producing image reference %w", err)
	}

	layer, err := tarball.LayerFromReader(strings.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("unexpected error adding task layer to image %w", err)
	}

	img, err := mutate.Append(empty.Image, mutate.Addendum{
		Layer: layer,
		Annotations: map[string]string{
			"org.opencontainers.image.title": key,
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
