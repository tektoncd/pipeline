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

const (
	maxControllerNotices = 10
	maxNoticeMessageLen  = 1024
	maxNoticeFileLen     = 256
)

// NoticeLevel indicates the severity of a notice.
type NoticeLevel string

const (
	// NoticeLevelInfo represents an informational message, no action needed.
	NoticeLevelInfo NoticeLevel = "info"
	// NoticeLevelWarning represents something to address, but not blocking.
	NoticeLevelWarning NoticeLevel = "warning"
)

// AllNoticeLevels can be used for NoticeLevel validation.
var AllNoticeLevels = []NoticeLevel{NoticeLevelInfo, NoticeLevelWarning}

// Notice represents a structured message emitted by a controller that does not affect the run's success/failure status.
type Notice struct {
	// Level indicates the severity of the notice.
	// Valid values: "info", "warning".
	// +kubebuilder:validation:Enum=info;warning
	Level NoticeLevel `json:"level"`

	// Message is the human-readable notice text.
	// Maximum length: 1024 characters.
	// +kubebuilder:validation:MaxLength=1024
	Message string `json:"message"`

	// Step is the name of the step that emitted this notice.
	// Empty for controller-emitted notices.
	// +optional
	Step string `json:"step,omitempty"`

	// File is the source file path related to this notice.
	// Used by VCS integrations to create inline annotations.
	// Maximum length: 256 characters.
	// +optional
	// +kubebuilder:validation:MaxLength=256
	File string `json:"file,omitempty"`

	// StartLine is the starting line number in the source file (1-based).
	// Pointer type so that absence (nil) is distinguishable from line 0.
	// +optional
	StartLine *int `json:"startLine,omitempty"`
}

// AddControllerNotice appends a bounded, deduplicated controller notice to TaskRun status.
func (trs *TaskRunStatus) AddControllerNotice(notice Notice) {
	if notice.Level != NoticeLevelInfo && notice.Level != NoticeLevelWarning {
		return
	}
	notice.Message = truncateString(notice.Message, maxNoticeMessageLen)
	notice.File = truncateString(notice.File, maxNoticeFileLen)
	notice.Step = ""

	for _, existing := range trs.Notices {
		if existing.Level == notice.Level && existing.Message == notice.Message {
			return
		}
	}
	if len(trs.Notices) >= maxControllerNotices {
		return
	}
	trs.Notices = append(trs.Notices, notice)
}

func truncateString(value string, limit int) string {
	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}
