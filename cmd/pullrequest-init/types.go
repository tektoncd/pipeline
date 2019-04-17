package main

// These are the generic resource types used for interacting with Pull Requests
// and related resources. They should not be tied to any particular SCM system.

// PullRequest represents a generic pull request resource.
type PullRequest struct {
	Type       string
	ID         int64
	Head, Base *GitReference
	Comments   []*Comment
	Labels     []*Label

	// Path to raw pull request payload.
	Raw string
}

// GitReference represents a git ref object. See
// https://git-scm.com/book/en/v2/Git-Internals-Git-References for more details.
type GitReference struct {
	Repo   string
	Branch string
	SHA    string
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
