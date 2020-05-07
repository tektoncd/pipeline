package fake

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jenkins-x/go-scm/scm"
	"k8s.io/apimachinery/pkg/util/sets"
)

type pullService struct {
	client *wrapper
	data   *Data
}

func (s *pullService) Find(ctx context.Context, repo string, number int) (*scm.PullRequest, *scm.Response, error) {
	f := s.data
	val, exists := f.PullRequests[number]
	if !exists {
		return nil, nil, fmt.Errorf("Pull request number %d does not exit", number)
	}
	if labels, _, err := s.client.Issues.ListLabels(ctx, repo, number, scm.ListOptions{}); err == nil {
		val.Labels = labels
	}
	return val, nil, nil
}

func (s *pullService) FindComment(context.Context, string, int, int) (*scm.Comment, *scm.Response, error) {
	panic("implement me")
}

func (s *pullService) List(context.Context, string, scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	panic("implement me")
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	f := s.data
	returnStart, returnEnd := paginated(opts.Page, opts.Size, len(f.PullRequestChanges[number]))
	return f.PullRequestChanges[number][returnStart:returnEnd], nil, nil
}

func (s *pullService) ListComments(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	f := s.data
	return append([]*scm.Comment{}, f.PullRequestComments[number]...), nil, nil
}

func (s *pullService) ListLabels(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	f := s.data
	re := regexp.MustCompile(fmt.Sprintf(`^%s#%d:(.*)$`, repo, number))
	la := []*scm.Label{}
	allLabels := sets.NewString(f.PullRequestLabelsExisting...)
	allLabels.Insert(f.PullRequestLabelsAdded...)
	allLabels.Delete(f.PullRequestLabelsRemoved...)
	for _, l := range allLabels.List() {
		groups := re.FindStringSubmatch(l)
		if groups != nil {
			la = append(la, &scm.Label{Name: groups[1]})
		}
	}
	return la, nil, nil
}

func (s *pullService) ListEvents(context.Context, string, int, scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	f := s.data
	labelString := fmt.Sprintf("%s#%d:%s", repo, number, label)
	if sets.NewString(f.PullRequestLabelsAdded...).Has(labelString) {
		return nil, fmt.Errorf("cannot add %v to %s/#%d", label, repo, number)
	}
	if f.RepoLabelsExisting == nil {
		f.PullRequestLabelsAdded = append(f.PullRequestLabelsAdded, labelString)
		return nil, nil
	}
	for _, l := range f.RepoLabelsExisting {
		if label == l {
			f.PullRequestLabelsAdded = append(f.PullRequestLabelsAdded, labelString)
			return nil, nil
		}
	}
	return nil, fmt.Errorf("cannot add %v to %s/#%d", label, repo, number)
}

// DeleteLabel removes a label
func (s *pullService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	f := s.data
	labelString := fmt.Sprintf("%s#%d:%s", repo, number, label)
	if !sets.NewString(f.PullRequestLabelsRemoved...).Has(labelString) {
		f.PullRequestLabelsRemoved = append(f.PullRequestLabelsRemoved, labelString)
		return nil, nil
	}
	return nil, fmt.Errorf("cannot remove %v from %s/#%d", label, repo, number)
}

func (s *pullService) Merge(context.Context, string, int, *scm.PullRequestMergeOptions) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) Close(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) CreateComment(ctx context.Context, repo string, number int, comment *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	f := s.data
	f.PullRequestCommentsAdded = append(f.PullRequestCommentsAdded, fmt.Sprintf("%s#%d:%s", repo, number, comment.Body))
	answer := &scm.Comment{
		ID:     f.IssueCommentID,
		Body:   comment.Body,
		Author: scm.User{Login: botName},
	}
	f.PullRequestComments[number] = append(f.PullRequestComments[number], answer)
	f.IssueCommentID++
	return answer, nil, nil
}

func (s *pullService) DeleteComment(ctx context.Context, repo string, number int, id int) (*scm.Response, error) {
	f := s.data
	f.PullRequestCommentsDeleted = append(f.PullRequestCommentsDeleted, fmt.Sprintf("%s#%d", repo, id))
	for num, ics := range f.PullRequestComments {
		for i, ic := range ics {
			if ic.ID == id {
				f.PullRequestComments[num] = append(ics[:i], ics[i+1:]...)
				return nil, nil
			}
		}
	}
	return nil, fmt.Errorf("could not find issue comment %d", id)
}

func (s *pullService) EditComment(ctx context.Context, repo string, number int, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) Create(ctx context.Context, repo string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	f := s.data
	f.PullRequestID++
	answer := &scm.PullRequest{
		Number: f.PullRequestID,
		Title:  input.Title,
		Body:   input.Body,
		Base: scm.PullRequestBranch{
			Ref: input.Base,
		},
		Head: scm.PullRequestBranch{
			Ref: input.Head,
		},
	}
	f.PullRequestsCreated[f.PullRequestID] = input
	f.PullRequests[f.PullRequestID] = answer
	return answer, nil, nil
}
