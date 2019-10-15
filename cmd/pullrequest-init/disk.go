/*
Copyright 2019 The Tekton Authors

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
	"encoding/gob"
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
)

// This file contains functions used to write a PR to disk. The representation is as follows:

// /workspace/
// /workspace/<resource>/
// /workspace/<resource>/labels/
// /workspace/<resource>/labels/<label>
// /workspace/<resource>/status/
// /workspace/<resource>/status/<status>.json
// /workspace/<resource>/comments/
// /workspace/<resource>/comments/<comment>.json
// /workspace/<resource>/head.json
// /workspace/<resource>/base.json

// Filenames for labels and statuses are URL encoded for safety.

const (
	manifestPath = ".MANIFEST"
)

// ToDisk converts a PullRequest object to an on-disk representation at the specified path.
func ToDisk(pr *PullRequest, path string) error {
	labelsPath := filepath.Join(path, "labels")
	commentsPath := filepath.Join(path, "comments")
	statusesPath := filepath.Join(path, "status")

	// Setup subdirs
	for _, p := range []string{labelsPath, commentsPath, statusesPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}

	// Start with comments
	if err := commentsToDisk(commentsPath, pr.Comments); err != nil {
		return err
	}

	// Now labels
	if err := labelsToDisk(labelsPath, pr.Labels); err != nil {
		return err
	}

	// Now status
	if err := statusToDisk(statusesPath, pr.Statuses); err != nil {
		return err
	}

	// Now refs
	if err := refToDisk("head", path, pr.Head); err != nil {
		return err
	}
	if err := refToDisk("base", path, pr.Base); err != nil {
		return err
	}

	return nil
}

func commentsToDisk(path string, comments []*Comment) error {
	manifest := Manifest{}
	for _, c := range comments {
		id := strconv.FormatInt(c.ID, 10)
		commentPath := filepath.Join(path, id+".json")
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(commentPath, b, 0600); err != nil {
			return err
		}
		manifest[id] = true
	}

	// Create a manifest to keep track of the comments that existed when the
	// resource was initialized. This is used to verify that a comment that
	// doesn't exist on disk was actually deleted by the user and not newly
	// created during upload.
	return manifestToDisk(manifest, filepath.Join(path, manifestPath))
}

func labelsToDisk(path string, labels []*Label) error {
	manifest := Manifest{}
	for _, l := range labels {
		name := url.QueryEscape(l.Text)
		labelPath := filepath.Join(path, name)
		if err := ioutil.WriteFile(labelPath, []byte{}, 0600); err != nil {
			return err
		}
		manifest[name] = true
	}
	return manifestToDisk(manifest, filepath.Join(path, manifestPath))
}

func statusToDisk(path string, statuses []*Status) error {
	for _, s := range statuses {
		statusName := url.QueryEscape(s.ID) + ".json"
		statusPath := filepath.Join(path, statusName)
		b, err := json.Marshal(s)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(statusPath, b, 0700); err != nil {
			return err
		}
	}
	return nil
}

func refToDisk(name, path string, r *GitReference) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(path, name+".json"), b, 0700)
}

// FromDisk outputs a PullRequest object from an on-disk representation at the specified path.
func FromDisk(path string) (*PullRequest, map[string]Manifest, error) {
	labelsPath := filepath.Join(path, "labels")
	commentsPath := filepath.Join(path, "comments")
	statusesPath := filepath.Join(path, "status")

	pr := PullRequest{}
	manifests := make(map[string]Manifest)

	var err error
	var manifest Manifest

	pr.Comments, manifest, err = commentsFromDisk(commentsPath)
	if err != nil {
		return nil, nil, err
	}
	manifests["comments"] = manifest

	pr.Labels, manifest, err = labelsFromDisk(labelsPath)
	if err != nil {
		return nil, nil, err
	}
	manifests["labels"] = manifest

	pr.Statuses, err = statusesFromDisk(statusesPath)
	if err != nil {
		return nil, nil, err
	}

	pr.Base, err = refFromDisk(path, "base.json")
	if err != nil {
		return nil, nil, err
	}
	pr.Head, err = refFromDisk(path, "head.json")
	if err != nil {
		return nil, nil, err
	}
	return &pr, manifests, nil
}

// Manifest is a list of sub-resources that exist within the PR resource to
// determine whether an item existed when the resource was initialized.
type Manifest map[string]bool

func manifestToDisk(manifest Manifest, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := gob.NewEncoder(f)
	return enc.Encode(manifest)
}

func manifestFromDisk(path string) (Manifest, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	m := Manifest{}
	dec := gob.NewDecoder(f)
	if err := dec.Decode(&m); err != nil {
		return nil, err
	}
	return m, nil
}

func commentsFromDisk(path string) ([]*Comment, Manifest, error) {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	comments := []*Comment{}
	for _, fi := range fis {
		if fi.Name() == manifestPath {
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return nil, nil, err
		}
		comment := Comment{}
		if err := json.Unmarshal(b, &comment); err != nil {
			// The comment might be plain text.
			comment.Text = string(b)
		}
		comments = append(comments, &comment)
	}

	manifest, err := manifestFromDisk(filepath.Join(path, manifestPath))
	if err != nil {
		return nil, nil, err
	}

	return comments, manifest, nil
}

func labelsFromDisk(path string) ([]*Label, Manifest, error) {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, nil, err
	}
	labels := []*Label{}
	for _, fi := range fis {
		if fi.Name() == manifestPath {
			continue
		}
		text, err := url.QueryUnescape(fi.Name())
		if err != nil {
			return nil, nil, err
		}
		labels = append(labels, &Label{Text: text})
	}

	manifest, err := manifestFromDisk(filepath.Join(path, manifestPath))
	if err != nil {
		return nil, nil, err
	}

	return labels, manifest, nil
}

func statusesFromDisk(path string) ([]*Status, error) {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}

	statuses := []*Status{}
	for _, fi := range fis {
		b, err := ioutil.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return nil, err
		}
		status := Status{}
		if err := json.Unmarshal(b, &status); err != nil {
			return nil, err
		}
		statuses = append(statuses, &status)
	}
	return statuses, nil
}

func refFromDisk(path, name string) (*GitReference, error) {
	b, err := ioutil.ReadFile(filepath.Join(path, name))
	if err != nil {
		return nil, err
	}
	ref := GitReference{}
	if err := json.Unmarshal(b, &ref); err != nil {
		return nil, err
	}
	return &ref, nil
}
