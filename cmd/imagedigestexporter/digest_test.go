package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/random"
)

func TestGetDigest(t *testing.T) {
	mustGetIndex := func(ii v1.ImageIndex, err error) v1.ImageIndex {
		if err != nil {
			t.Fatalf("must get image: %s", err)
		}
		return ii
	}
	mustGetManifest := func(im *v1.IndexManifest, err error) *v1.IndexManifest {
		if err != nil {
			t.Fatalf("must get manifest: %s", err)
		}
		return im
	}
	mustGetDigest := func(h v1.Hash, err error) v1.Hash {
		if err != nil {
			t.Fatalf("must get digest: %s", err)
		}
		return h
	}
	indexSingleImage := mustGetIndex(random.Index(1024, 4, 1))
	indexMultipleImages := mustGetIndex(random.Index(1024, 4, 4))
	tests := []struct {
		name     string
		index    v1.ImageIndex
		expected v1.Hash
	}{
		{
			name:     "empty index",
			index:    empty.Index,
			expected: mustGetDigest(empty.Index.Digest()),
		},
		{
			name:     "index with single image",
			index:    indexSingleImage,
			expected: mustGetManifest(indexSingleImage.IndexManifest()).Manifests[0].Digest,
		},
		{
			name:     "index with multiple images",
			index:    indexMultipleImages,
			expected: mustGetDigest(indexMultipleImages.Digest()),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			digest, err := GetDigest(test.index)
			if err != nil {
				t.Fatalf("cannot get digest: %s", err)
			}
			if diff := cmp.Diff(digest, test.expected); diff != "" {
				t.Errorf("get digest: -want +got: %s", diff)
			}
		})
	}
}
