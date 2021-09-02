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

package pullrequest

import (
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

// This file contains functions used to write a PR to disk. The representation is as follows:

// /workspace/
// /workspace/<resource>/
// /workspace/<resource>/pr.json
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

// Resource represents a complete SCM resource that should be recorded to disk.
type Resource struct {
	PR       *scm.PullRequest
	Statuses []*scm.Status
	Comments []*scm.Comment

	// Manifests contain data about the resource when it was written to disk.
	Manifests map[string]Manifest
}

// ToDisk converts a PullRequest object to an on-disk representation at the
// specified path. When written, the underlying Manifests
func ToDisk(r *Resource, path string) error {
	labelsPath := filepath.Join(path, "labels")
	commentsPath := filepath.Join(path, "comments")
	statusesPath := filepath.Join(path, "status")

	// Setup subdirs
	for _, p := range []string{labelsPath, commentsPath, statusesPath} {
		if err := os.MkdirAll(p, 0755); err != nil {
			return err
		}
	}

	// Make PR read-only. Users should use corresponding subresources if they
	// want to interact with the PR.
	if err := toDisk(filepath.Join(path, "pr.json"), r.PR, 0400); err != nil {
		return err
	}

	if err := commentsToDisk(commentsPath, r.Comments); err != nil {
		return err
	}

	if err := labelsToDisk(labelsPath, r.PR.Labels); err != nil {
		return err
	}

	if err := statusToDisk(statusesPath, r.Statuses); err != nil {
		return err
	}

	// Now refs
	if err := refToDisk("head", path, r.PR.Head); err != nil {
		return err
	}
	return refToDisk("base", path, r.PR.Base)
}

func commentsToDisk(path string, comments []*scm.Comment) error {
	manifest := Manifest{}
	for _, c := range comments {
		id := strconv.Itoa(c.ID)
		commentPath := filepath.Join(path, id+".json")
		if err := toDisk(commentPath, c, 0600); err != nil {
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

func labelsToDisk(path string, labels []*scm.Label) error {
	manifest := Manifest{}
	for _, l := range labels {
		name := url.QueryEscape(l.Name)
		labelPath := filepath.Join(path, name)
		if err := ioutil.WriteFile(labelPath, []byte{}, 0600); err != nil {
			return err
		}
		manifest[name] = true
	}
	return manifestToDisk(manifest, filepath.Join(path, manifestPath))
}

func statusToDisk(path string, statuses []*scm.Status) error {
	for _, s := range statuses {
		statusName := url.QueryEscape(s.Label) + ".json"
		statusPath := filepath.Join(path, statusName)
		if err := toDisk(statusPath, s, 0600); err != nil {
			return err
		}
	}
	return nil
}

func refToDisk(name, path string, r scm.PullRequestBranch) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filepath.Join(path, name+".json"), b, 0600)
}

func toDisk(path string, r interface{}, perm os.FileMode) error {
	b, err := json.Marshal(r)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(path, b, perm)
}

// FromDisk outputs a PullRequest object from an on-disk representation at the specified path.
func FromDisk(path string, disableJSONComments bool) (*Resource, error) {
	r := &Resource{
		PR:        &scm.PullRequest{},
		Manifests: make(map[string]Manifest),
	}

	var err error
	var manifest Manifest

	// If pr.json exists in output directory, preload r.PR
	prPath := filepath.Join(path, "pr.json")
	if _, err := os.Stat(prPath); err == nil {
		b, err := ioutil.ReadFile(prPath)
		if err != nil {
			return nil, err
		}
		if err := json.Unmarshal(b, r.PR); err != nil {
			return nil, err
		}
	}

	commentsPath := filepath.Join(path, "comments")
	r.Comments, manifest, err = commentsFromDisk(commentsPath, disableJSONComments)
	if err != nil {
		return nil, err
	}
	r.Manifests["comments"] = manifest

	labelsPath := filepath.Join(path, "labels")
	r.PR.Labels, manifest, err = labelsFromDisk(labelsPath)
	if err != nil {
		return nil, err
	}
	r.Manifests["labels"] = manifest

	statusesPath := filepath.Join(path, "status")
	r.Statuses, err = statusesFromDisk(statusesPath)
	if err != nil {
		return nil, err
	}

	r.PR.Base, err = refFromDisk(path, "base.json")
	if err != nil {
		return nil, err
	}
	r.PR.Head, err = refFromDisk(path, "head.json")
	if err != nil {
		return nil, err
	}

	if r.PR.Head.Sha != "" {
		r.PR.Sha = r.PR.Head.Sha
	}

	return r, nil
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

func commentsFromDisk(path string, disableStrictJSON bool) ([]*scm.Comment, Manifest, error) {
	fis, err := ioutil.ReadDir(path)
	if isNotExistError(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	comments := []*scm.Comment{}
	for _, fi := range fis {
		if fi.Name() == manifestPath {
			continue
		}
		b, err := ioutil.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return nil, nil, err
		}
		var comment scm.Comment
		if strings.EqualFold(filepath.Ext(fi.Name()), ".json") {
			if err := json.Unmarshal(b, &comment); err != nil {
				if disableStrictJSON {
					comment.Body = string(b)
				} else {
					return nil, nil, fmt.Errorf("error parsing comment file %q: %w", fi.Name(), err)
				}
			}
		} else {
			comment.Body = string(b)
		}
		comments = append(comments, &comment)
	}

	manifest, err := manifestFromDisk(filepath.Join(path, manifestPath))
	if err != nil {
		return nil, nil, err
	}

	return comments, manifest, nil
}

func labelsFromDisk(path string) ([]*scm.Label, Manifest, error) {
	fis, err := ioutil.ReadDir(path)
	if isNotExistError(err) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	labels := []*scm.Label{}
	for _, fi := range fis {
		if fi.Name() == manifestPath {
			continue
		}
		text, err := url.QueryUnescape(fi.Name())
		if err != nil {
			return nil, nil, err
		}
		labels = append(labels, &scm.Label{Name: text})
	}

	manifest, err := manifestFromDisk(filepath.Join(path, manifestPath))
	if err != nil {
		return nil, nil, err
	}

	return labels, manifest, nil
}

func statusesFromDisk(path string) ([]*scm.Status, error) {
	fis, err := ioutil.ReadDir(path)
	if isNotExistError(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	statuses := []*scm.Status{}
	for _, fi := range fis {
		b, err := ioutil.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return nil, err
		}
		status := scm.Status{}
		if err := json.Unmarshal(b, &status); err != nil {
			return nil, err
		}
		statuses = append(statuses, &status)
	}
	return statuses, nil
}

func refFromDisk(path, name string) (scm.PullRequestBranch, error) {
	b, err := ioutil.ReadFile(filepath.Join(path, name))
	if err != nil {
		return scm.PullRequestBranch{}, err
	}
	ref := scm.PullRequestBranch{}
	if err := json.Unmarshal(b, &ref); err != nil {
		return scm.PullRequestBranch{}, err
	}
	return ref, nil
}

func isNotExistError(err error) bool {
	return err != nil && os.IsNotExist(err)
}
