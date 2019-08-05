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
package common

import (
	"fmt"
	"os"
	"strings"
)

// ManifestItem describes an item in the source manifest.
type ManifestItem struct {
	// SourceURL is the URL of the object in Cloud Storage.
	SourceURL string `json:"sourceUrl"`

	// Sha1Sum is the SHA1 digest of the object.
	Sha1Sum string `json:"sha1sum"`

	// FileMode is the mode of the file that should be applied to the
	// fetched file.
	FileMode os.FileMode `json:"mode"`
}

// ParseBucketObject parses a URI into the bucket and object name it points to.
//
// It supports URIs in either of these forms:
// - https://storage.googleapis.com/bucket/path/to/object
// - gs://bucket/path/to/object
//
// In both of the above cases, bucket=bucket and object=path/to/object.
func ParseBucketObject(uri string) (bucket, object string, generation int64, err error) {
	// TODO: Parse generation.
	switch {
	case strings.HasPrefix(uri, "https://storage.googleapis.com/") || strings.HasPrefix(uri, "http://storage.googleapis.com/"):
		// uri looks like "https://storage.googleapis.com/staging.my-project.appspot.com/3aa080e5e72a610b06033dbfee288483d87cfd61"
		if parts := strings.Split(uri, "/"); len(parts) >= 5 {
			bucket := parts[3]
			object := strings.Join(parts[4:], "/")
			return bucket, object, 0, nil
		}
	case strings.HasPrefix(uri, "gs://"):
		// uri looks like "gs://my-bucket/manifest-20171004T175409.json"
		if parts := strings.Split(uri, "/"); len(parts) >= 4 {
			bucket := parts[2]
			object := strings.Join(parts[3:], "/")
			return bucket, object, 0, nil
		}
	}
	return "", "", 0, fmt.Errorf("cannot parse bucket/object from uri %q", uri)
}
