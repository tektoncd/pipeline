// Copyright 2017 Drone.IO Inc. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package scm

import (
	"encoding/json"
	"fmt"
	"strings"
)

// State represents the commit state.
type State int

// State values.
const (
	StateUnknown State = iota
	StateFailure
	StateCanceled
	StateError
	StateExpected
	StatePending
	StateRunning
	StateSuccess
)

// String returns a string representation of the State
func (s State) String() string {
	switch s {
	case StateUnknown:
		return "unknown"
	case StatePending:
		return "pending"
	case StateRunning:
		return "running"
	case StateSuccess:
		return "success"
	case StateFailure:
		return "failure"
	case StateCanceled:
		return "cancelled"
	case StateExpected:
		return "expected"
	case StateError:
		return "error"
	default:
		return "unknown"
	}
}

// ToState converts the given text to a state
func ToState(s string) State {
	switch strings.ToLower(s) {
	case "pending":
		return StatePending
	case "running":
		return StateRunning
	case "success":
		return StateSuccess
	case "failure":
		return StateFailure
	case "cancelled":
		return StateCanceled
	case "expected":
		return StateExpected
	case "error":
		return StateError
	default:
		return StateUnknown
	}
}

// MarshalJSON marshals State to JSON
func (s State) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`%q`, s.String())), nil
}

// UnmarshalJSON unmarshals JSON to State
func (s *State) UnmarshalJSON(b []byte) error {
	*s = ToState(strings.Trim(string(b), `"`))
	return nil
}

// Action identifies webhook actions.
type Action int

// Action values.
const (
	ActionCreate Action = iota + 1
	ActionUpdate
	ActionDelete
	// issues
	ActionOpen
	ActionReopen
	ActionClose
	ActionLabel
	ActionUnlabel
	// pull requests
	ActionSync
	ActionMerge
	ActionAssigned
	ActionUnassigned
	ActionReviewRequested
	ActionReviewRequestRemoved
	ActionReadyForReview
	ActionConvertedToDraft
	// reviews
	ActionEdited
	ActionSubmitted
	ActionDismissed
	// check run / check suite
	ActionCompleted
)

// String returns the string representation of Action.
func (a Action) String() (s string) {
	switch a {
	case ActionCreate:
		return "created"
	case ActionUpdate:
		return "updated"
	case ActionDelete:
		return "deleted"
	case ActionLabel:
		return "labeled"
	case ActionUnlabel:
		return "unlabeled"
	case ActionOpen:
		return "opened"
	case ActionReopen:
		return "reopened"
	case ActionClose:
		return "closed"
	case ActionSync:
		return "synchronized"
	case ActionMerge:
		return "merged"
	case ActionEdited:
		return "edited"
	case ActionSubmitted:
		return "submitted"
	case ActionDismissed:
		return "dismisssed"
	case ActionAssigned:
		return "assigned"
	case ActionUnassigned:
		return "unassigned"
	case ActionReviewRequested:
		return "review_requested"
	case ActionReviewRequestRemoved:
		return "review_request_removed"
	case ActionReadyForReview:
		return "ready_for_review"
	case ActionConvertedToDraft:
		return "converted_to_draft"
	case ActionCompleted:
		return "completed"
	default:
		return
	}
}

// MarshalJSON returns the JSON-encoded Action.
func (a Action) MarshalJSON() ([]byte, error) {
	return json.Marshal(a.String())
}

// UnmarshalJSON unmarshales the JSON-encoded Action.
func (a *Action) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	switch s {
	case "created":
		*a = ActionCreate
	case "updated":
		*a = ActionUpdate
	case "deleted":
		*a = ActionDelete
	case "labeled":
		*a = ActionLabel
	case "unlabeled":
		*a = ActionUnlabel
	case "opened":
		*a = ActionOpen
	case "reopened":
		*a = ActionReopen
	case "closed":
		*a = ActionClose
	case "synchronized":
		*a = ActionSync
	case "merged":
		*a = ActionMerge
	case "completed":
		*a = ActionCompleted
	case "ready_for_review":
		*a = ActionReadyForReview
	case "converted_to_draft":
		*a = ActionConvertedToDraft
	case "submitted":
		*a = ActionSubmitted
	case "dismissed":
		*a = ActionDismissed
	case "edited":
		*a = ActionEdited
	}
	return nil
}

// Driver identifies source code management driver.
type Driver int

// Driver values.
const (
	DriverUnknown Driver = iota
	DriverGithub
	DriverGitlab
	DriverGogs
	DriverGitea
	DriverBitbucket
	DriverStash
	DriverCoding
	DriverFake
	DriverAzure
)

// String returns the string representation of Driver.
func (d Driver) String() (s string) {
	switch d {
	case DriverGithub:
		return "github"
	case DriverGitlab:
		return "gitlab"
	case DriverGogs:
		return "gogs"
	case DriverGitea:
		return "gitea"
	case DriverBitbucket:
		return "bitbucket"
	case DriverStash:
		return "stash"
	case DriverCoding:
		return "coding"
	case DriverFake:
		return "fake"
	case DriverAzure:
		return "azure"
	default:
		return "unknown"
	}
}

// SearchTimeFormat is a time.Time format string for ISO8601 which is the
// format that GitHub requires for times specified as part of a search query.
const SearchTimeFormat = "2006-01-02T15:04:05Z"
