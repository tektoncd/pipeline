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
	"encoding/json"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jenkins-x/go-scm/scm"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestToDisk(t *testing.T) {
	rsrc := &Resource{
		PR: &scm.PullRequest{
			Number: 123,
			Head: scm.PullRequestBranch{
				Ref:  "refs/heads/branch1",
				Sha:  "sha1",
				Repo: scm.Repository{Name: "repo1"},
			},
			Base: scm.PullRequestBranch{
				Ref:  "refs/heads/branch1",
				Sha:  "sha2",
				Repo: scm.Repository{Name: "repo1"},
			},
			Labels: []*scm.Label{
				{Name: "help"},
				{Name: "me"},
				{Name: "foo/bar"},
			},
		},
		Statuses: []*scm.Status{
			{
				Label:  "123",
				State:  scm.StateSuccess,
				Desc:   "foobar",
				Target: "https://foo.bar",
			},
			{
				Label:  "cla/foo",
				State:  scm.StateSuccess,
				Desc:   "bazbat",
				Target: "https://baz.bat",
			},
		},
		Comments: []*scm.Comment{{
			ID:   123,
			Body: "hey",
		}},
	}

	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)
	if err := ToDisk(rsrc, d); err != nil {
		t.Error(err)
	}

	// Base PR
	pr := &scm.PullRequest{}
	readAndUnmarshal(t, filepath.Join(d, "pr.json"), pr)
	if err != nil {
		t.Fatal(err)
	}
	if d := cmp.Diff(pr, rsrc.PR); d != "" {
		t.Errorf("PullRequest %s", diff.PrintWantGot(d))
	}

	// Check the refs
	checkRef := func(name string, r scm.PullRequestBranch) {
		actualRef := scm.PullRequestBranch{}
		readAndUnmarshal(t, filepath.Join(d, name), &actualRef)
		if d := cmp.Diff(actualRef, r); d != "" {
			t.Errorf("Get PullRequest %s", diff.PrintWantGot(d))
		}
	}
	checkRef("head.json", rsrc.PR.Head)
	checkRef("base.json", rsrc.PR.Base)

	// Check the Statuses
	fis, err := ioutil.ReadDir(filepath.Join(d, "status"))
	if err != nil {
		t.Fatal(err)
	}

	statuses := map[string]scm.Status{}
	for _, fi := range fis {
		status := scm.Status{}
		readAndUnmarshal(t, filepath.Join(d, "status", fi.Name()), &status)
		statuses[status.Target] = status
	}
	for _, s := range rsrc.Statuses {
		actualStatus, ok := statuses[s.Target]
		if !ok {
			t.Errorf("Expected status with ID: %s, not found: %v", s.Target, statuses)
		}
		if d := cmp.Diff(actualStatus, *s); d != "" {
			t.Errorf("Get Status %s", diff.PrintWantGot(d))
		}
	}
	// Check the labels
	fis, err = ioutil.ReadDir(filepath.Join(d, "labels"))
	if err != nil {
		t.Fatal(err)
	}
	labels := map[string]struct{}{}
	labelManifest := Manifest{}
	for _, fi := range fis {
		if fi.Name() == manifestPath {
			continue
		}
		text, err := url.QueryUnescape(fi.Name())
		if err != nil {
			t.Errorf("Error decoding label text: %s", fi.Name())
		}
		labels[text] = struct{}{}
		labelManifest[fi.Name()] = true
	}

	for _, l := range rsrc.PR.Labels {
		if _, ok := labels[l.Name]; !ok {
			t.Errorf("Expected label with text: %s, not found: %v", l.Name, labels)
		}
	}
	gotManifest, err := manifestFromDisk(filepath.Join(d, "labels", manifestPath))
	if err != nil {
		t.Fatalf("Error reading comment manifest: %v", err)
	}
	for m := range gotManifest {
		if !labelManifest[m] {
			t.Errorf("Label %s not found in manifest: %+v", m, labelManifest)
		}
	}
	if len(labelManifest) != len(gotManifest) {
		t.Errorf("Label manifest does not match expected length. expected %d, got %d: %+v", len(labelManifest), len(gotManifest), gotManifest)
	}

	// Check the comments
	fis, err = ioutil.ReadDir(filepath.Join(d, "comments"))
	if err != nil {
		t.Fatal(err)
	}

	comments := map[int]scm.Comment{}
	commentManifest := Manifest{}
	for _, fi := range fis {
		if fi.Name() == manifestPath {
			continue
		}
		comment := scm.Comment{}
		path := filepath.Join(d, "comments", fi.Name())
		readAndUnmarshal(t, path, &comment)
		comments[comment.ID] = comment
		commentManifest[strconv.Itoa(comment.ID)] = true
	}
	for _, c := range rsrc.Comments {
		actualComment, ok := comments[c.ID]
		if !ok {
			t.Errorf("Expected comment with ID: %d, not found: %v", c.ID, comments)
		}
		if d := cmp.Diff(actualComment, *c); d != "" {
			t.Errorf("Get Comment %s", diff.PrintWantGot(d))
		}
	}
	gotManifest, err = manifestFromDisk(filepath.Join(d, "comments", manifestPath))
	if err != nil {
		t.Fatalf("Error reading comment manifest: %v", err)
	}
	for m := range gotManifest {
		if !commentManifest[m] {
			t.Errorf("Comment %s not found in manifest", m)
		}
	}
	if len(commentManifest) != len(gotManifest) {
		t.Errorf("Comment manifest does not match expected length. Got %+v", gotManifest)
	}
}

func TestFromDiskWithoutComments(t *testing.T) {
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	// Write some refs
	base := scm.PullRequestBranch{
		Repo: scm.Repository{Name: "repo1"},
		Ref:  "refs/heads/branch1",
		Sha:  "sha1",
	}
	head := scm.PullRequestBranch{
		Repo: scm.Repository{Name: "repo2"},
		Ref:  "refs/heads/branch2",
		Sha:  "sha2",
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

	rsrc, err := FromDisk(d, false)
	if err != nil {
		t.Fatal(err)
	}

	// Check the refs
	if d := cmp.Diff(rsrc.PR.Base, base); d != "" {
		t.Errorf("Get Base %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(rsrc.PR.Head, head); d != "" {
		t.Errorf("Get Head %s", diff.PrintWantGot(d))
	}

}

func TestFromDisk(t *testing.T) {
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	// Write some refs
	base := scm.PullRequestBranch{
		Repo: scm.Repository{Name: "repo1"},
		Ref:  "refs/heads/branch1",
		Sha:  "sha1",
	}
	head := scm.PullRequestBranch{
		Repo: scm.Repository{Name: "repo2"},
		Ref:  "refs/heads/branch2",
		Sha:  "sha2",
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
	statuses := []scm.Status{
		{
			Label: "abc",
			Desc:  "foo",
			State: scm.StateSuccess,
		},
		{
			Label: "def",
			Desc:  "bar",
			State: scm.StateFailure,
		},
	}

	if err := os.MkdirAll(filepath.Join(d, "status"), 0750); err != nil {
		t.Fatal(err)
	}
	for _, s := range statuses {
		writeFile(filepath.Join(d, "status", s.Label+".json"), &s)
	}

	// Write some labels
	if err := os.MkdirAll(filepath.Join(d, "labels"), 0750); err != nil {
		t.Fatal(err)
	}
	labels := []string{"hey", "you", "size%2Flgtm"}
	for _, l := range labels {
		if err := ioutil.WriteFile(filepath.Join(d, l), []byte{}, 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeManifest(t, labels, filepath.Join(d, "labels", manifestPath))

	// Write some comments
	comments := []scm.Comment{
		{
			Body:   "testing",
			Author: scm.User{Login: "me"},
			ID:     123,
		},
		{
			Body:   "1212",
			Author: scm.User{Login: "you"},
			ID:     234,
		},
	}

	if err := os.MkdirAll(filepath.Join(d, "comments"), 0750); err != nil {
		t.Fatal(err)
	}
	manifest := make([]string, 0, len(comments))
	for _, c := range comments {
		id := strconv.Itoa(c.ID)
		writeFile(filepath.Join(d, "comments", id+".json"), &c)
		manifest = append(manifest, id)
	}
	writeManifest(t, manifest, filepath.Join(d, "comments", manifestPath))

	// Comments can also be plain text.
	if err := ioutil.WriteFile(filepath.Join(d, "comments", "plain"), []byte("plaincomment"), 0700); err != nil {
		t.Fatal(err)
	}

	rsrc, err := FromDisk(d, false)
	if err != nil {
		t.Fatal(err)
	}

	// Check the refs
	if d := cmp.Diff(rsrc.PR.Base, base); d != "" {
		t.Errorf("Get Base %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(rsrc.PR.Head, head); d != "" {
		t.Errorf("Get Head %s", diff.PrintWantGot(d))
	}
	if d := cmp.Diff(rsrc.PR.Sha, head.Sha); d != "" {
		t.Errorf("Get Sha %s", diff.PrintWantGot(d))
	}

	// Check the comments
	commentMap := map[int]scm.Comment{}
	for _, c := range comments {
		commentMap[c.ID] = c
	}
	commentMap[0] = scm.Comment{
		Body: "plaincomment",
	}
	for _, c := range rsrc.Comments {
		if d := cmp.Diff(commentMap[c.ID], *c); d != "" {
			t.Errorf("Get comments %s", diff.PrintWantGot(d))
		}
	}
	commentManifest := Manifest{}
	for _, c := range comments {
		commentManifest[strconv.Itoa(c.ID)] = true
	}
	if d := cmp.Diff(commentManifest, rsrc.Manifests["comments"]); d != "" {
		t.Errorf("Comment manifest %s", diff.PrintWantGot(d))
	}

	// Check the labels
	labelsMap := map[string]struct{}{}
	for _, l := range labels {
		labelsMap[l] = struct{}{}
	}
	for _, l := range rsrc.PR.Labels {
		key := url.QueryEscape(l.Name)
		if d := cmp.Diff(labelsMap[key], &l); d != "" {
			t.Errorf("Get labels %s", diff.PrintWantGot(d))
		}
	}
	labelManifest := Manifest{}
	for _, l := range labels {
		labelManifest[l] = true
	}
	if d := cmp.Diff(labelManifest, rsrc.Manifests["labels"]); d != "" {
		t.Errorf("Label manifest %s", diff.PrintWantGot(d))
	}

	// Check the statuses
	statusMap := map[string]scm.Status{}
	for _, s := range statuses {
		statusMap[s.Label] = s
	}
	for _, s := range rsrc.Statuses {
		if d := cmp.Diff(statusMap[s.Label], *s); d != "" {
			t.Errorf("Get status %s", diff.PrintWantGot(d))
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

func TestCommentsFromDisk(t *testing.T) {
	tests := []struct {
		name              string
		files             map[string][]byte
		disableStrictJSON bool
		expectErr         bool
		expectedComments  []*scm.Comment
	}{
		{
			name: "comments from .json and plain text files",
			files: map[string][]byte{
				"comment_foo.json": []byte(`
{
  "Body": "foo",
  "Author": {
    "Login": "author"
  }
}`),
				"comment_bar": []byte("bar"),
			},
			expectedComments: []*scm.Comment{
				{
					Body: "bar",
				},
				{
					Body:   "foo",
					Author: scm.User{Login: "author"},
				},
			},
		},
		{
			name: "any non-json extension is considered text",
			files: map[string][]byte{
				"comment.ANY": []byte("comment_any"),
			},
			expectedComments: []*scm.Comment{
				{
					Body: "comment_any",
				},
			},
		},
		{
			name: "invalid content in .json file",
			files: map[string][]byte{
				"comment.json": []byte("invalid_json"),
			},
			expectErr: true,
		},
		{
			name: "JSON extension is case-insensitive",
			files: map[string][]byte{
				"comment.JSON": []byte("invalid_json"),
			},
			expectErr: true,
		},
		{
			name: "JSON extension parsing can be disabled using `disable-json-comments`",
			files: map[string][]byte{
				"comment.JSON": []byte("comment"),
			},
			disableStrictJSON: true,
			expectedComments: []*scm.Comment{
				{
					Body: "comment",
				},
			},
		},
		{
			name: "valid JSON files are parsed as such even with `disable-json-comments`",
			files: map[string][]byte{
				"comment_foo.json": []byte(`
{
  "Body": "foo",
  "Author": {
    "Login": "author"
  }
}`),
			},
			disableStrictJSON: true,
			expectedComments: []*scm.Comment{
				{
					Body:   "foo",
					Author: scm.User{Login: "author"},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(d)
			if err := os.MkdirAll(filepath.Join(d, "comments"), 0750); err != nil {
				t.Fatal(err)
			}
			fileNames := []string{}
			for fileName, contents := range tt.files {
				fileNames = append(fileNames, fileName)
				if err := ioutil.WriteFile(filepath.Join(d, "comments", fileName), contents, 0700); err != nil {
					t.Fatal(err)
				}
			}
			writeManifest(t, fileNames, filepath.Join(d, "comments", manifestPath))

			comments, _, err := commentsFromDisk(filepath.Join(d, "comments"), tt.disableStrictJSON)
			if err == nil && tt.expectErr {
				t.Fatal("expected error, got nil")
			}
			if err != nil && !tt.expectErr {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(comments) != len(tt.expectedComments) {
				t.Fatalf("Comments length does not match expected length. expected %d, got %d: %+v", len(tt.expectedComments), len(comments), comments)
			}
			for i, c := range tt.expectedComments {
				if d := cmp.Diff(c, comments[i]); d != "" {
					t.Errorf("Get comments: %s", diff.PrintWantGot(d))
				}
			}
		})
	}
}

func TestLabelsToDisk(t *testing.T) {
	type args struct {
		labels []*scm.Label
	}
	tests := []struct {
		name      string
		args      args
		wantFiles []string
	}{
		{
			name: "single label",
			args: args{
				labels: []*scm.Label{
					{Name: "foo"},
				},
			},
			wantFiles: []string{
				"foo",
			},
		},
		{
			name: "multiple labels",
			args: args{
				labels: []*scm.Label{
					{Name: "foo"},
					{Name: "bar"},
				},
			},
			wantFiles: []string{
				"foo",
				"bar",
			},
		},
		{
			name: "complex labels",
			args: args{
				labels: []*scm.Label{
					{Name: "foo/bar"},
					{Name: "help wanted"},
					{Name: "simple"},
				},
			},
			wantFiles: []string{
				"foo%2Fbar",
				"help+wanted",
				"simple",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("Error creating temp dir: %s", err)
			}
			defer os.RemoveAll(d)
			if err := labelsToDisk(d, tt.args.labels); err != nil {
				t.Errorf("labelsToDisk() error = %v", err)
			}
			for _, f := range tt.wantFiles {
				if _, err := os.Stat(filepath.Join(d, f)); err != nil {
					t.Errorf("expected file %s to exist", f)
				}
			}
		})
	}
}

func TestStatusToDisk(t *testing.T) {
	type args struct {
		statuses []*scm.Status
	}
	tests := []struct {
		name      string
		args      args
		wantFiles []string
	}{
		{
			name: "single status",
			args: args{
				statuses: []*scm.Status{
					{Label: "foo"},
				},
			},
			wantFiles: []string{
				"foo.json",
			},
		},
		{
			name: "multiple statuses",
			args: args{
				statuses: []*scm.Status{
					{Label: "foo"},
					{Label: "bar"},
				},
			},
			wantFiles: []string{
				"foo.json",
				"bar.json",
			},
		},
		{
			name: "complex statuses",
			args: args{
				statuses: []*scm.Status{
					{Label: "foo/bar"},
					{Label: "help wanted"},
					{Label: "simple"},
				},
			},
			wantFiles: []string{
				"foo%2Fbar.json",
				"help+wanted.json",
				"simple.json",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("Error creating temp dir: %s", err)
			}
			defer os.RemoveAll(d)
			if err := statusToDisk(d, tt.args.statuses); err != nil {
				t.Errorf("statusToDisk() error = %v", err)
			}
			for _, f := range tt.wantFiles {
				if _, err := os.Stat(filepath.Join(d, f)); err != nil {
					t.Errorf("expected file %s to exist", f)
				}
			}
		})
	}
}

func writeManifest(t *testing.T, items []string, path string) {
	t.Helper()
	m := Manifest{}
	for _, i := range items {
		m[i] = true
	}
	if err := manifestToDisk(m, path); err != nil {
		t.Fatal(err)
	}
}

func TestLabelsFromDisk(t *testing.T) {
	type args struct {
		fileNames []string
	}
	tests := []struct {
		name string
		args args
		want []scm.Label
	}{
		{
			name: "single label",
			args: args{
				fileNames: []string{"foo"},
			},
			want: []scm.Label{
				{Name: "foo"},
			},
		},
		{
			name: "multiple labels",
			args: args{
				fileNames: []string{"foo", "bar"},
			},
			want: []scm.Label{
				{Name: "foo"},
				{Name: "bar"},
			},
		},
		{
			name: "complex labels",
			args: args{
				fileNames: []string{"foo%2Fbar", "bar+bat"},
			},
			want: []scm.Label{
				{Name: "foo/bar"},
				{Name: "bar bat"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d, err := ioutil.TempDir("", "")
			if err != nil {
				t.Fatalf("Error creating temp dir: %s", err)
			}
			defer os.RemoveAll(d)

			for _, l := range tt.args.fileNames {
				if err := ioutil.WriteFile(filepath.Join(d, l), []byte{}, 0700); err != nil {
					t.Errorf("Error creating label: %s", err)
				}
			}
			writeManifest(t, tt.args.fileNames, filepath.Join(d, manifestPath))
			got, _, err := labelsFromDisk(d)
			if err != nil {
				t.Errorf("labelsFromDisk() error = %v", err)
			}

			derefed := []scm.Label{}
			for _, l := range got {
				derefed = append(derefed, *l)
			}

			sort.Slice(derefed, func(i, j int) bool {
				return derefed[i].Name < derefed[j].Name
			})
			sort.Slice(tt.want, func(i, j int) bool {
				return tt.want[i].Name < tt.want[j].Name
			})

			if !reflect.DeepEqual(derefed, tt.want) {
				t.Errorf("labelsFromDisk() = %v, want %v", derefed, tt.want)
			}
		})
	}
}

func TestFromDiskPRShaWithNullHeadAndBase(t *testing.T) {
	d, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(d)

	expectedSha := "1a2s3d4f5g6g6h7j8k9l"
	// Write some refs
	base := scm.PullRequestBranch{
		Repo: scm.Repository{},
		Ref:  "",
		Sha:  "",
	}
	head := scm.PullRequestBranch{
		Repo: scm.Repository{},
		Ref:  "",
		Sha:  "",
	}
	pr := scm.PullRequest{
		Sha:  expectedSha,
		Base: base,
		Head: head,
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
	writeFile(filepath.Join(d, "pr.json"), &pr)

	rsrc, err := FromDisk(d, false)
	if err != nil {
		t.Fatal(err)
	}

	if rsrc.PR.Sha != expectedSha {
		t.Errorf("FromDisk() returned sha `%s`, expected `%s`", rsrc.PR.Sha, expectedSha)
	}

}
