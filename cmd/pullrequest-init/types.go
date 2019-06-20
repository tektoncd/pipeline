package main

// These are the generic resource types used for interacting with Pull Requests
// and related resources. They should not be tied to any particular SCM system.

type StatusCode string

// TODO: Figure out how to do case insensitive statuses.
const (
	Unknown        StatusCode = "unknown"
	Success        StatusCode = "success"
	Failure        StatusCode = "failure"
	Error          StatusCode = "error"
	Neutral        StatusCode = "neutral"
	Queued         StatusCode = "queued"
	InProgress     StatusCode = "in_progress"
	Timeout        StatusCode = "timeout"
	Canceled       StatusCode = "canceled"
	ActionRequired StatusCode = "action_required"
)

// TODO: Add getters to make types nil-safe.

// PullRequest represents a generic pull request resource.
type PullRequest struct {
	Type       string
	ID         int64
	Head, Base *GitReference
	Statuses   []*Status
	Comments   []*Comment
	Labels     []*Label

	// Path to raw pull request payload.
	Raw string
	// Path to raw status payload.
	RawStatus string
}

// GitReference represents a git ref object. See
// https://git-scm.com/book/en/v2/Git-Internals-Git-References for more details.
type GitReference struct {
	Repo   string
	Branch string
	SHA    string
}

type Status struct {
	// ID uniquely distinguish this status from other status types.
	ID string
	// Code defines the status of the status.
	Code StatusCode
	// Short summary of the status.
	Description string
	// Where the status should link to.
	URL string
}

// Comment represents a pull request comment.
type Comment struct {
	Text   string
	Author string
	ID     int64
	Raw    string
}

// Label represents a Pull Request Label
type Label struct {
	Text string
}
