package gogs

import (
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type release struct {
	ID          int       `json:"id"`
	Title       string    `json:"name"`
	Description string    `json:"body"`
	Tag         string    `json:"tag_name"`
	Commitish   string    `json:"target_commitish"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	Author      *user     `json:"author"`
	Created     time.Time `json:"created_at"`
}

func convertRelease(from *release) *scm.Release {
	return &scm.Release{
		ID:          from.ID,
		Title:       from.Title,
		Description: from.Description,
		Tag:         from.Tag,
		Commitish:   from.Commitish,
		Draft:       from.Draft,
		Prerelease:  from.Prerelease,
		Created:     from.Created,
		Published:   from.Created,
	}
}
