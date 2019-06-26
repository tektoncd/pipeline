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

// GetType returns the Type field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetType() string {
	if pr == nil {
		return ""
	}
	return pr.Type
}

// GetID returns the ID field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetID() int64 {
	if pr == nil {
		return 0
	}
	return pr.ID
}

// GetHead returns the Head field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetHead() *GitReference {
	if pr == nil {
		return nil
	}
	return pr.Head
}

// GetBase returns the Base field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetBase() *GitReference {
	if pr == nil {
		return nil
	}
	return pr.Base
}

// GetComments returns the Comments field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetComments() []*Comment {
	if pr == nil {
		return nil
	}
	return pr.Comments
}

// GetLabels returns the Labels field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetLabels() []*Label {
	if pr == nil {
		return nil
	}
	return pr.Labels
}

// GetRaw returns the Raw field, or zero value if PullRequest is nil.
func (pr *PullRequest) GetRaw() string {
	if pr == nil {
		return ""
	}
	return pr.Raw
}

// GitReference represents a git ref object. See
// https://git-scm.com/book/en/v2/Git-Internals-Git-References for more details.
type GitReference struct {
	Repo   string
	Branch string
	SHA    string
}

// GetRepo returns the Repo field, or zero value if GitReference is nil.
func (r *GitReference) GetRepo() string {
	if r == nil {
		return ""
	}
	return r.Repo
}

// GetBranch returns the Branch field, or zero value if GitReference is nil.
func (r *GitReference) GetBranch() string {
	if r == nil {
		return ""
	}
	return r.Branch
}

// GetSHA returns the SHA field, or zero value if GitReference is nil.
func (r *GitReference) GetSHA() string {
	if r == nil {
		return ""
	}
	return r.SHA
}

// Comment represents a pull request comment.
type Comment struct {
	Text   string
	Author string
	ID     int64
	Raw    string
}

func (c *Comment) GetText() string {
	if c == nil {
		return ""
	}
	return c.Text
}

// GetAuthor returns the Author field, or zero value if Comment is nil.
func (c *Comment) GetAuthor() string {
	if c == nil {
		return ""
	}
	return c.Author
}

// GetID returns the ID field, or zero value if Comment is nil.
func (c *Comment) GetID() int64 {
	if c == nil {
		return 0
	}
	return c.ID
}

// GetRaw returns the Raw field, or zero value if Comment is nil.
func (c *Comment) GetRaw() string {
	if c == nil {
		return ""
	}
	return c.Raw
}

// Label represents a Pull Request Label
type Label struct {
	Text string
}

// GetText returns the Text field, or zero value if Label is nil.
func (l *Label) GetText() string {
	if l == nil {
		return ""
	}
	return l.Text
}
