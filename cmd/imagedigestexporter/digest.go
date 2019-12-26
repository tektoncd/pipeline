package main

import (
	v1 "github.com/google/go-containerregistry/pkg/v1"
)

// GetDigest returns the digest of an OCI image index. If there is only one image in the index, the
// digest of the image is returned; otherwise, the digest of the whole index is returned.
func GetDigest(ii v1.ImageIndex) (v1.Hash, error) {
	im, err := ii.IndexManifest()
	if err != nil {
		return v1.Hash{}, err
	}
	if len(im.Manifests) == 1 {
		return im.Manifests[0].Digest, nil
	}
	return ii.Digest()
}
