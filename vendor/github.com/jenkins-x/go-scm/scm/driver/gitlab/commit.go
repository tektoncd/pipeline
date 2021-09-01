package gitlab

import (
	"context"
	"fmt"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type commitService struct {
	client *wrapper
}

func (s *commitService) UpdateCommitStatus(ctx context.Context,
	repo string, sha string, options scm.CommitStatusUpdateOptions) (*scm.CommitStatus, *scm.Response, error) {
	path := fmt.Sprintf("api/v4/projects/%s/statuses/%s", encode(repo), sha)

	out := new(commitStatus)
	opts := convertCommitStatusUpdateOptions(options)

	res, err := s.client.do(ctx, "POST", path, opts, out)
	return convertCommitStatus(out), res, err
}

func convertCommitStatusUpdateOptions(from scm.CommitStatusUpdateOptions) commitStatusUpdateOptions {
	return commitStatusUpdateOptions{
		ID:          from.ID,
		Sha:         from.Sha,
		State:       from.State,
		Ref:         from.Ref,
		Name:        from.Name,
		TargetURL:   from.TargetURL,
		Description: from.Description,
		Coverage:    from.Coverage,
		PipelineID:  from.PipelineID,
	}
}

func convertCommitStatus(from *commitStatus) *scm.CommitStatus {
	return &scm.CommitStatus{
		Status:       from.Status,
		Created:      from.Created,
		Started:      from.Started,
		Name:         from.Name,
		AllowFailure: from.AllowFailure,
		Author: scm.CommitStatusAuthor{
			Username:  from.Author.Username,
			WebURL:    from.Author.WebURL,
			AvatarURL: from.Author.AvatarURL,
			ID:        from.Author.ID,
			State:     from.Author.State,
			Name:      from.Author.Name,
		},
		Description: from.Description,
		Sha:         from.Sha,
		TargetURL:   from.TargetURL,
		ID:          from.ID,
		Ref:         from.Ref,
		Coverage:    from.Coverage,
	}
}

type commitStatusUpdateOptions struct {
	ID          string  `json:"id"`
	Sha         string  `json:"sha"`
	State       string  `json:"state"`
	Ref         string  `json:"ref"`
	Name        string  `json:"name"`
	TargetURL   string  `json:"target_url"`
	Description string  `json:"description"`
	Coverage    float64 `json:"coverage"`
	PipelineID  *int    `json:"pipeline_id,omitempty"`
}

type commitStatus struct {
	Status       string             `json:"status"`
	Created      time.Time          `json:"created_at"`
	Started      time.Time          `json:"updated_at"`
	Name         string             `json:"name"`
	AllowFailure bool               `json:"allow_failure"`
	Author       commitStatusAuthor `json:"author"`
	Description  string             `json:"description"`
	Sha          string             `json:"sha"`
	TargetURL    string             `json:"target_url"`
	Finished     time.Time          `json:"finished_at"`
	ID           int                `json:"id"`
	Ref          string             `json:"ref"`
	Coverage     float64            `json:"coverage"`
}

type commitStatusAuthor struct {
	Username  string `json:"username"`
	State     string `json:"state"`
	WebURL    string `json:"web_url"`
	AvatarURL string `json:"avatar_url"`
	ID        int    `json:"id"`
	Name      string `json:"name"`
}
