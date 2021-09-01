package fake

import (
	"context"
	"fmt"
	"regexp"

	"github.com/jenkins-x/go-scm/scm"
	"k8s.io/apimachinery/pkg/util/sets"
)

type issueService struct {
	client *wrapper
	data   *Data
}

const botName = "k8s-ci-robot"

func (s *issueService) Search(context.Context, scm.SearchOptions) ([]*scm.SearchIssue, *scm.Response, error) {
	// TODO implemment
	return nil, nil, nil
}

func (s *issueService) ListEvents(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	f := s.data
	return append([]*scm.ListedIssueEvent{}, f.IssueEvents[number]...), nil, nil
}

func (s *issueService) Find(ctx context.Context, repo string, number int) (*scm.Issue, *scm.Response, error) {
	f := s.data
	for _, slice := range f.Issues {
		for _, issue := range slice {
			if issue.Number == number {
				return issue, nil, nil
			}
		}
	}
	return nil, nil, nil
}

func (s *issueService) ListLabels(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
	f := s.data
	re := regexp.MustCompile(fmt.Sprintf(`^%s#%d:(.*)$`, repo, number))
	la := []*scm.Label{}
	allLabels := sets.NewString(f.IssueLabelsExisting...)
	allLabels.Insert(f.IssueLabelsAdded...)
	allLabels.Delete(f.IssueLabelsRemoved...)
	for _, l := range allLabels.List() {
		groups := re.FindStringSubmatch(l)
		if groups != nil {
			la = append(la, &scm.Label{Name: groups[1]})
		}
	}
	return la, nil, nil
}

func (s *issueService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	f := s.data
	labelString := fmt.Sprintf("%s#%d:%s", repo, number, label)
	if sets.NewString(f.IssueLabelsAdded...).Has(labelString) {
		return nil, fmt.Errorf("cannot add %v to %s/#%d", label, repo, number)
	}
	if f.RepoLabelsExisting == nil {
		f.IssueLabelsAdded = append(f.IssueLabelsAdded, labelString)
		return nil, nil
	}
	for _, l := range f.RepoLabelsExisting {
		if label == l {
			f.IssueLabelsAdded = append(f.IssueLabelsAdded, labelString)
			return nil, nil
		}
	}
	return nil, fmt.Errorf("cannot add %v to %s/#%d", label, repo, number)
}

// DeleteLabel removes a label
func (s *issueService) DeleteLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	f := s.data
	labelString := fmt.Sprintf("%s#%d:%s", repo, number, label)
	if !sets.NewString(f.IssueLabelsRemoved...).Has(labelString) {
		f.IssueLabelsRemoved = append(f.IssueLabelsRemoved, labelString)
		return nil, nil
	}
	return nil, fmt.Errorf("cannot remove %v from %s/#%d", label, repo, number)
}

// FindIssues returns f.Issues
func (s *issueService) FindIssues(query, sort string, asc bool) ([]scm.Issue, error) {
	f := s.data
	var issues []scm.Issue
	for _, slice := range f.Issues {
		for _, issue := range slice {
			issues = append(issues, *issue)
		}
	}
	return issues, nil
}

// AssignIssue adds assignees.
func (s *issueService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	f := s.data
	var m scm.MissingUsers
	for _, a := range logins {
		if a == "not-in-the-org" {
			m.Users = append(m.Users, a)
			continue
		}
		f.AssigneesAdded = append(f.AssigneesAdded, fmt.Sprintf("%s#%d:%s", repo, number, a))
	}
	if m.Users == nil {
		return nil, nil
	}
	return nil, m
}

func (s *issueService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) FindComment(context.Context, string, int, int) (*scm.Comment, *scm.Response, error) {
	panic("implement me")
}

func (s *issueService) List(context.Context, string, scm.IssueListOptions) ([]*scm.Issue, *scm.Response, error) {
	panic("implement me")
}

func (s *issueService) ListComments(ctx context.Context, repo string, number int, opts scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	f := s.data
	return append([]*scm.Comment{}, f.IssueComments[number]...), nil, nil
}

func (s *issueService) Create(context.Context, string, *scm.IssueInput) (*scm.Issue, *scm.Response, error) {
	panic("implement me")
}

func (s *issueService) CreateComment(ctx context.Context, repo string, number int, comment *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	f := s.data
	f.IssueCommentsAdded = append(f.IssueCommentsAdded, fmt.Sprintf("%s#%d:%s", repo, number, comment.Body))
	answer := &scm.Comment{
		ID:     f.IssueCommentID,
		Body:   comment.Body,
		Author: scm.User{Login: botName},
	}
	f.IssueComments[number] = append(f.IssueComments[number], answer)
	f.IssueCommentID++
	return answer, nil, nil
}

func (s *issueService) DeleteComment(ctx context.Context, repo string, number int, id int) (*scm.Response, error) {
	f := s.data
	f.IssueCommentsDeleted = append(f.IssueCommentsDeleted, fmt.Sprintf("%s#%d", repo, id))
	for num, ics := range f.IssueComments {
		for i, ic := range ics {
			if ic.ID == id {
				f.IssueComments[num] = append(ics[:i], ics[i+1:]...)
				return nil, nil
			}
		}
	}
	return nil, fmt.Errorf("could not find issue comment %d", id)
}

func (s *issueService) EditComment(ctx context.Context, repo string, number int, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *issueService) Close(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) Reopen(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) Lock(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) Unlock(context.Context, string, int) (*scm.Response, error) {
	panic("implement me")
}

func (s *issueService) SetMilestone(ctx context.Context, repo string, issueID int, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *issueService) ClearMilestone(ctx context.Context, repo string, id int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}
