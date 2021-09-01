package scm

import (
	"context"
	"time"
)

type (
	// CommitStatus for commit status
	CommitStatus struct {
		Status       string
		Created      time.Time
		Started      time.Time
		Name         string
		AllowFailure bool
		Author       CommitStatusAuthor
		Description  string
		Sha          string
		TargetURL    string
		Finished     time.Time
		ID           int
		Ref          string
		Coverage     float64
	}

	// CommitStatusAuthor for commit author
	CommitStatusAuthor struct {
		Username  string
		State     string
		WebURL    string
		AvatarURL string
		ID        int
		Name      string
	}

	// CommitStatusUpdateOptions for update options
	CommitStatusUpdateOptions struct {
		ID          string
		Sha         string
		State       string
		Ref         string
		Name        string
		TargetURL   string
		Description string
		Coverage    float64
		PipelineID  *int
	}
)

// CommitService commit interface
type CommitService interface {
	UpdateCommitStatus(ctx context.Context,
		repo string, sha string, options CommitStatusUpdateOptions) (*CommitStatus, *Response, error)
}
