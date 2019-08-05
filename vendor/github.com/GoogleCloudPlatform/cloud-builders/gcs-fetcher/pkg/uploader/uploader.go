/*
Copyright 2018 Google, Inc. All rights reserved.

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
package uploader

import (
	"context"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"google.golang.org/api/googleapi"

	"github.com/GoogleCloudPlatform/cloud-builders/gcs-fetcher/pkg/common"
)

// Uploader encapsulates methods for uploading files incrementally and
// producing a source manifest.
type Uploader struct {
	gcs                    GCS
	os                     OS
	bucket, manifestObject string

	manifest                 sync.Map
	totalBytes, bytesSkipped int64
}

// OS allows us to inject dependencies to facilitate testing.
type OS interface {
	EvalSymlinks(path string) (string, error)
	Stat(path string) (os.FileInfo, error)
}

// GCS allows us to inject dependencies to facilitate testing.
type GCS interface {
	NewWriter(ctx context.Context, bucket, object string) io.WriteCloser
}

type job struct {
	path string
	info os.FileInfo
}

// New returns a new Uploader.
func New(ctx context.Context, gcs GCS, os OS, bucket, manifestObject string, numWorkers int) *Uploader {
	return &Uploader{
		gcs:            gcs,
		os:             os,
		bucket:         bucket,
		manifestObject: manifestObject,
	}
}

// Wait blocks until ongoing uploads are complete, or until an error is
// encountered.
func (u *Uploader) Done(ctx context.Context) error {
	uploaded := u.totalBytes - u.bytesSkipped
	var incr float64
	if u.totalBytes != 0 {
		incr = float64(100 * u.bytesSkipped / u.totalBytes)
	}
	fmt.Printf(`
******************************************************
* Uploaded %d bytes (%.2f%% incremental)
******************************************************
`, uploaded, incr)
	return u.writeManifest(ctx)
}

func (u *Uploader) Do(ctx context.Context, path string, info os.FileInfo) error {
	// Follow symlinks.
	if spath, err := u.os.EvalSymlinks(path); err != nil {
		return err
	} else if spath != path {
		info, err = u.os.Stat(spath)
		if err != nil {
			return err
		}
		path = spath
	}

	// Don't process dirs.
	if info.IsDir() {
		return nil
	}

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	// Compute digest of file, and count bytes.
	cw := &countWriter{}
	h := sha1.New()
	if _, err := io.Copy(io.MultiWriter(cw, h), f); err != nil {
		return err
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))

	// Seek back to the beginning of the file, to write it to GCS.
	// NB: The GCS client is responsible for skipping writes if the file
	// already exists.
	if _, err := f.Seek(0, 0); err != nil {
		return err
	}
	wc := u.gcs.NewWriter(ctx, u.bucket, digest)
	if _, err := io.Copy(wc, f); err != nil {
		return err
	}

	u.manifest.Store(path, common.ManifestItem{
		SourceURL: fmt.Sprintf("gs://%s/%s", u.bucket, digest),
		Sha1Sum:   digest,
		FileMode:  info.Mode(),
	})

	if err := wc.Close(); isAlreadyExists(err) {
		u.bytesSkipped += cw.b
	} else if err != nil {
		return err
	}
	u.totalBytes += cw.b
	return nil
}

type countWriter struct {
	b int64
}

func (c *countWriter) Write(b []byte) (int, error) {
	c.b += int64(len(b))
	return len(b), nil
}

func isAlreadyExists(err error) bool {
	if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == http.StatusPreconditionFailed {
		return true
	}
	return false
}

func (u *Uploader) writeManifest(ctx context.Context) error {
	m := map[string]common.ManifestItem{}
	u.manifest.Range(func(k, v interface{}) bool {
		m[k.(string)] = v.(common.ManifestItem)
		return true
	})

	wc := u.gcs.NewWriter(ctx, u.bucket, u.manifestObject)
	if err := json.NewEncoder(wc).Encode(m); err != nil {
		return err
	}
	if err := wc.Close(); err != nil {
		return err
	}
	fmt.Printf("Wrote manifest object gs://%s/%s", u.bucket, u.manifestObject)
	return nil
}
