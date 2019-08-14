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
	for _, c := range comments {
		commentPath := filepath.Join(path, strconv.FormatInt(c.ID, 10)+".json")
		b, err := json.Marshal(c)
		if err != nil {
			return err
		}
		if err := ioutil.WriteFile(commentPath, b, 0700); err != nil {
			return err
		}
	}
	return nil
}

func labelsToDisk(path string, labels []*Label) error {
	for _, l := range labels {
		name := url.QueryEscape(l.Text)
		labelPath := filepath.Join(path, name)
		if err := ioutil.WriteFile(labelPath, []byte{}, 0700); err != nil {
			return err
		}
	}
	return nil
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
func FromDisk(path string) (*PullRequest, error) {
	labelsPath := filepath.Join(path, "labels")
	commentsPath := filepath.Join(path, "comments")
	statusesPath := filepath.Join(path, "status")

	pr := PullRequest{}

	var err error

	// Start with comments
	pr.Comments, err = commentsFromDisk(commentsPath)
	if err != nil {
		return nil, err
	}

	// Now Labels
	pr.Labels, err = labelsFromDisk(labelsPath)
	if err != nil {
		return nil, err
	}

	// Now statuses
	pr.Statuses, err = statusesFromDisk(statusesPath)
	if err != nil {
		return nil, err
	}

	// Finally refs

	pr.Base, err = refFromDisk(path, "base.json")
	if err != nil {
		return nil, err
	}
	pr.Head, err = refFromDisk(path, "head.json")
	if err != nil {
		return nil, err
	}
	return &pr, nil
}

func commentsFromDisk(path string) ([]*Comment, error) {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	comments := []*Comment{}
	for _, fi := range fis {
		b, err := ioutil.ReadFile(filepath.Join(path, fi.Name()))
		if err != nil {
			return nil, err
		}
		comment := Comment{}
		if err := json.Unmarshal(b, &comment); err != nil {
			// The comment might be plain text.
			comment.Text = string(b)
		}
		comments = append(comments, &comment)
	}
	return comments, nil
}

func labelsFromDisk(path string) ([]*Label, error) {
	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return nil, err
	}
	labels := []*Label{}
	for _, fi := range fis {
		text, err := url.QueryUnescape(fi.Name())
		if err != nil {
			return nil, err
		}
		labels = append(labels, &Label{Text: text})
	}
	return labels, nil
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
