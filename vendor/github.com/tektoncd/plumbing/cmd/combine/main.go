/*
 Copyright 2021 The Tekton Authors

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
	"fmt"
	"log"
	"os"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func main() {
	if len(os.Args) != 4 {
		log.Fatal("expected exactly 3 args (src1, src2, dst)")
	}

	src1, src2, dst := os.Args[1], os.Args[2], os.Args[3]
	log.Println("combining", src1, src2, "into", dst)
	dstr, err := name.ParseReference(dst)
	if err != nil {
		log.Fatal(err)
	}

	pull := func(s string) (v1.ImageIndex, error) {
		r, err := name.ParseReference(s)
		if err != nil {
			return nil, err
		}
		return remote.Index(r)
	}
	src1i, err := pull(src1)
	if err != nil {
		log.Fatalf("pulling %q: %v", src1, err)
	}
	src2i, err := pull(src2)
	if err != nil {
		log.Fatalf("pulling %q: %v", src2, err)
	}

	plats := map[string]bool{}
	var adds []mutate.IndexAddendum
	add := func(idx v1.ImageIndex) error {
		mf, err := idx.IndexManifest()
		if err != nil {
			return fmt.Errorf("IndexManifest: %w", err)
		}
		for _, desc := range mf.Manifests {
			if desc.Platform == nil {
				return fmt.Errorf("found nil platform for manifest %s", desc.Digest)
			}

			b, err := json.Marshal(desc.Platform)
			if err != nil {
				return fmt.Errorf("marshalling platform: %w", err)
			}
			if plats[string(b)] {
				return fmt.Errorf("conflicting platform %+v", *desc.Platform)
			}
			plats[string(b)] = true
			log.Printf("found platform %+v", *desc.Platform)

			img, err := idx.Image(desc.Digest)
			if err != nil {
				return fmt.Errorf("getting image %s: %w", desc.Digest, err)
			}
			adds = append(adds, mutate.IndexAddendum{
				Add:        img,
				Descriptor: desc,
			})
		}
		return nil
	}
	log.Println("---", src1, "---")
	if err := add(src1i); err != nil {
		log.Fatalf("adding manifests from src1: %v", err)
	}
	log.Println("---", src2, "---")
	if err := add(src2i); err != nil {
		log.Fatalf("adding manifests from src2: %v", err)
	}

	dsti := mutate.AppendManifests(mutate.IndexMediaType(empty.Index, types.DockerManifestList), adds...)
	mf, err := dsti.IndexManifest()
	if err != nil {
		log.Fatal(err)
	}
	b, err := json.MarshalIndent(mf, "", " ")
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Pushing manifest:", string(b))

	log.Println("pushing...")
	if err := remote.WriteIndex(dstr, dsti, remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		log.Fatalf("pushing %q: %v", dst, err)
	}
	log.Println("pushed")

	d, err := dsti.Digest()
	if err != nil {
		log.Fatalf("digest: %v", err)
	}

	fmt.Print(dstr.Context().Digest(d.String()))
}
