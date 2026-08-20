/*
Copyright 2026 The Tekton Authors

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

package v1

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/tektoncd/pipeline/test/diff"
)

func TestTaskRunStatusAddControllerNotice(t *testing.T) {
	longMessage := strings.Repeat("m", 1100)
	longFile := strings.Repeat("f", 300)
	trs := &TaskRunStatus{}

	trs.AddControllerNotice(Notice{Level: NoticeLevelWarning, Message: "warn", Step: "ignored"})
	trs.AddControllerNotice(Notice{Level: NoticeLevelWarning, Message: "warn"})
	trs.AddControllerNotice(Notice{Level: NoticeLevelInfo, Message: longMessage, File: longFile})
	trs.AddControllerNotice(Notice{Level: NoticeLevel("bad"), Message: "drop"})

	want := []Notice{{
		Level:   NoticeLevelWarning,
		Message: "warn",
	}, {
		Level:   NoticeLevelInfo,
		Message: strings.Repeat("m", 1024),
		File:    strings.Repeat("f", 256),
	}}
	if d := cmp.Diff(want, trs.Notices); d != "" {
		t.Fatal(diff.PrintWantGot(d))
	}
}

func TestTaskRunStatusAddControllerNoticeCaps(t *testing.T) {
	trs := &TaskRunStatus{}
	for i := range maxControllerNotices + 1 {
		trs.AddControllerNotice(Notice{Level: NoticeLevelWarning, Message: string(rune('a' + i))})
	}
	if got := len(trs.Notices); got != maxControllerNotices {
		t.Fatalf("notices length = %d, want %d", got, maxControllerNotices)
	}
}
