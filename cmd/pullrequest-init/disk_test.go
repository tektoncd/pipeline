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
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestToDisk(t *testing.T) {
	tektonPr := PullRequest{
		Type: "github",
		ID:   123,
		Head: &GitReference{
			Repo:   "foo1",
			Branch: "branch1",
			SHA:    "sha1",
		},
		Base: &GitReference{
			Repo:   "foo2",
			Branch: "branch2",
			SHA:    "sha2",
		},
		Statuses: []*Status{
			{
				ID:          "123",
				Code:        Success,
				Description: "foobar",
				URL:         "https://foo.bar",
			},
		},
		Comments: []*Comment{
			{
				Text:   "hey",
				Author: "me",
				ID:     123,
			},
		},
		Labels: []*Label{
			{Text: "help"},
			{Text: "me"},
		},
	}

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	if err := ToDisk(&tektonPr, d); err != nil {
		t.Error(err)
	}

	// Check the refs
	checkRef := func(name string, r GitReference) {
		actualRef := GitReference{}
		readAndUnmarshal(t, filepath.Join(d, name), &actualRef)
		if diff := cmp.Diff(actualRef, r); diff != "" {
			t.Errorf("Get PullRequest: -want +got: %s", diff)
		}
	}
	checkRef("head.json", *tektonPr.Head)
	checkRef("base.json", *tektonPr.Base)

	// Check the Statuses
	fis, err := ioutil.ReadDir(filepath.Join(d, "status"))
	if err != nil {
		t.Fatal(err)
	}

	statuses := map[string]Status{}
	for _, fi := range fis {
		status := Status{}
		readAndUnmarshal(t, filepath.Join(d, "status", fi.Name()), &status)
		statuses[status.ID] = status
	}
	for _, s := range tektonPr.Statuses {
		actualStatus, ok := statuses[s.ID]
		if !ok {
			t.Errorf("Expected status with ID: %s, not found: %v", s.ID, statuses)
		}
		if diff := cmp.Diff(actualStatus, *s); diff != "" {
			t.Errorf("Get Status: -want +got: %s", diff)
		}
	}

	// Check the labels
	fis, err = ioutil.ReadDir(filepath.Join(d, "labels"))
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]struct{}{}
	for _, fi := range fis {
		labels[fi.Name()] = struct{}{}
	}

	for _, l := range tektonPr.Labels {
		if _, ok := labels[l.Text]; !ok {
			t.Errorf("Expected label with text: %s, not found: %v", l.Text, labels)
		}
	}

	// Check the comments
	fis, err = ioutil.ReadDir(filepath.Join(d, "comments"))
	if err != nil {
		t.Fatal(err)
	}

	comments := map[int64]Comment{}
	for _, fi := range fis {
		comment := Comment{}
		readAndUnmarshal(t, filepath.Join(d, "comments", fi.Name()), &comment)
		comments[comment.ID] = comment
	}
	for _, c := range tektonPr.Comments {
		actualComment, ok := comments[c.ID]
		if !ok {
			t.Errorf("Expected comment with ID: %d, not found: %v", c.ID, comments)
		}
		if diff := cmp.Diff(actualComment, *c); diff != "" {
			t.Errorf("Get Comment: -want +got: %s", diff)
		}
	}
}

func TestFromDisk(t *testing.T) {
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	// Write some refs
	base := GitReference{
		Repo:   "repo1",
		Branch: "branch1",
		SHA:    "sha1",
	}
	head := GitReference{
		Repo:   "repo2",
		Branch: "branch2",
		SHA:    "sha2",
	}
	writeFile := func(p string, v interface{}) {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if err := ioutil.WriteFile(p, b, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(filepath.Join(d, "base.json"), &base)
	writeFile(filepath.Join(d, "head.json"), &head)

	// Write some statuses
	statuses := []Status{
		{
			ID:          "abc",
			Description: "foo",
			Code:        Success,
		},
		{
			ID:          "def",
			Description: "bar",
			Code:        Failure,
		},
	}

	if err := os.MkdirAll(filepath.Join(d, "status"), 0750); err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		writeFile(filepath.Join(d, "status", s.ID+".json"), &s)
	}

	// Write some labels
	if err := os.MkdirAll(filepath.Join(d, "labels"), 0750); err != nil {
		t.Fatal(err)
	}
	labels := []string{"hey", "you"}
	for _, l := range labels {
		if err := ioutil.WriteFile(filepath.Join(d, l), []byte{}, 0700); err != nil {
			t.Fatal(err)
		}
	}

	// Write some comments
	comments := []Comment{
		{
			Text:   "testing",
			Author: "me",
			ID:     123,
		},
		{
			Text:   "1212",
			Author: "you",
			ID:     234,
		},
	}

	if err := os.MkdirAll(filepath.Join(d, "comments"), 0750); err != nil {
		t.Fatal(err)
	}
	for _, c := range comments {
		writeFile(filepath.Join(d, "comments", strconv.FormatInt(c.ID, 10)+".json"), &c)
	}

	// Comments can also be plain text.
	if err := ioutil.WriteFile(filepath.Join(d, "comments", "plain"), []byte("plaincomment"), 0700); err != nil {
		t.Fatal(err)
	}

	pr, err := FromDisk(d)
	if err != nil {
		t.Fatal(err)
	}

	// Check the refs
	if diff := cmp.Diff(pr.Base, &base); diff != "" {
		t.Errorf("Get Base: -want +got: %s", diff)
	}
	if diff := cmp.Diff(pr.Head, &head); diff != "" {
		t.Errorf("Get Head: -want +got: %s", diff)
	}

	// Check the comments
	commentMap := map[int64]Comment{}
	for _, c := range comments {
		commentMap[c.ID] = c
	}
	commentMap[0] = Comment{
		Text: "plaincomment",
	}
	for _, c := range pr.Comments {
		if diff := cmp.Diff(commentMap[c.ID], *c); diff != "" {
			t.Errorf("Get comments: -want +got: %s", diff)
		}
	}

	// Check the labels
	labelsMap := map[string]struct{}{}
	for _, l := range labels {
		labelsMap[l] = struct{}{}
	}
	for _, l := range pr.Labels {
		if diff := cmp.Diff(labelsMap[l.Text], &l); diff != "" {
			t.Errorf("Get labels: -want +got: %s", diff)
		}
	}

	// Check the statuses
	statusMap := map[string]Status{}
	for _, s := range statuses {
		statusMap[s.ID] = s
	}
	for _, s := range pr.Statuses {
		if diff := cmp.Diff(statusMap[s.ID], *s); diff != "" {
			t.Errorf("Get status: -want +got: %s", diff)
		}
	}
}

func readAndUnmarshal(t *testing.T, p string, v interface{}) {
	t.Helper()
	b, err := ioutil.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatal(err)
	}
}
