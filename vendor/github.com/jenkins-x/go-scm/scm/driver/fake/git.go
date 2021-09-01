package fake

import (
	"context"
	"fmt"
	"strings"

	"github.com/jenkins-x/go-scm/scm"
)

type gitService struct {
	client *wrapper
	data   *Data
}

func (s *gitService) FindRef(ctx context.Context, repo, ref string) (string, *scm.Response, error) {
	f := s.data
	return f.TestRef, nil, nil
}

func (s *gitService) CreateRef(ctx context.Context, repo, ref, sha string) (*scm.Reference, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) DeleteRef(ctx context.Context, repo, ref string) (*scm.Response, error) {
	f := s.data
	paths := strings.SplitN(repo, "/", 2)
	if len(paths) < 2 {
		return nil, fmt.Errorf("repository string '%s' should contain two words separated by '/'", repo)
	}
	org := paths[0]
	name := paths[1]
	f.RefsDeleted = append(f.RefsDeleted, DeletedRef{Org: org, Repo: name, Ref: ref})
	return nil, nil
}

func (s *gitService) FindBranch(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) FindCommit(ctx context.Context, repo, SHA string) (*scm.Commit, *scm.Response, error) {
	f := s.data
	return f.Commits[SHA], nil, nil
}

func (s *gitService) FindTag(ctx context.Context, repo, name string) (*scm.Reference, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) ListBranches(ctx context.Context, repo string, opts scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) ListCommits(ctx context.Context, repo string, opts scm.CommitListOptions) ([]*scm.Commit, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) ListChanges(ctx context.Context, repo, ref string, opts scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) CompareCommits(ctx context.Context, repo, ref1, ref2 string, opts scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	panic("implement me")
}

func (s *gitService) ListTags(ctx context.Context, repo string, opts scm.ListOptions) ([]*scm.Reference, *scm.Response, error) {
	panic("implement me")
}
