package fake

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

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
		return nil, nil, fmt.Errorf("pull request number %d does not exit", number)
	}
	return val, nil, nil
}

func (s *pullService) FindComment(context.Context, string, int, int) (*scm.Comment, *scm.Response, error) {
	panic("implement me")
}

func (s *pullService) List(ctx context.Context, fullName string, opts *scm.PullRequestListOptions) ([]*scm.PullRequest, *scm.Response, error) {
	var allPullRequests []*scm.PullRequest
	f := s.data

	keys := make([]int, 0, len(f.PullRequests))
	for prKey := range f.PullRequests {
		keys = append(keys, prKey)
	}
	sort.Ints(keys)
	for _, prKey := range keys {
		pr := f.PullRequests[prKey]
		repo := pr.Repository()
		fn := repo.FullName
		if fn == "" {
			fn = scm.Join(repo.Namespace, repo.Name)
		}
		if fn == fullName {
			allPullRequests = append(allPullRequests, pr)
		}
	}

	filteredPullRequests := filterPullRequests(allPullRequests, opts)

	returnStart, returnEnd := paginated(opts.Page, opts.Size, len(filteredPullRequests))
	return filteredPullRequests[returnStart:returnEnd], nil, nil
}

func filterPullRequests(requests []*scm.PullRequest, opts *scm.PullRequestListOptions) []*scm.PullRequest {
	var filteredPullRequests, newFilteredPullRequests []int

	for i := 0; i < len(requests); i++ {
		filteredPullRequests = append(filteredPullRequests, i)
	}

	for _, i := range filteredPullRequests {
		if opts.Closed && requests[i].Closed {
			newFilteredPullRequests = append(newFilteredPullRequests, i)
		}

		if opts.Open && !requests[i].Closed {
			newFilteredPullRequests = append(newFilteredPullRequests, i)
		}
	}

	filteredPullRequests = newFilteredPullRequests
	newFilteredPullRequests = nil

	if opts.CreatedBefore != nil {
		for _, i := range filteredPullRequests {
			if requests[i].Created.Before(*opts.CreatedBefore) {
				newFilteredPullRequests = append(newFilteredPullRequests, i)
			}
		}

		filteredPullRequests = newFilteredPullRequests
		newFilteredPullRequests = nil
	}

	if opts.UpdatedBefore != nil {
		for _, i := range filteredPullRequests {
			if requests[i].Updated.Before(*opts.UpdatedBefore) {
				newFilteredPullRequests = append(newFilteredPullRequests, i)
			}
		}

		filteredPullRequests = newFilteredPullRequests
		newFilteredPullRequests = nil
	}

	if opts.CreatedAfter != nil {
		for _, i := range filteredPullRequests {
			if requests[i].Created.After(*opts.CreatedAfter) {
				newFilteredPullRequests = append(newFilteredPullRequests, i)
			}
		}

		filteredPullRequests = newFilteredPullRequests
		newFilteredPullRequests = nil
	}

	if opts.UpdatedAfter != nil {
		for _, i := range filteredPullRequests {
			if requests[i].Updated.After(*opts.UpdatedAfter) {
				newFilteredPullRequests = append(newFilteredPullRequests, i)
			}
		}

		filteredPullRequests = newFilteredPullRequests
	}

	// Filtering on labels is not implemented in all real providers either...
	// if len(opts.Labels) > 0 {
	// 	panic("implement me")
	// }

	returnRequests := []*scm.PullRequest{}

	for _, i := range filteredPullRequests {
		returnRequests = append(returnRequests, requests[i])
	}

	return returnRequests
}

func (s *pullService) ListChanges(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Change, *scm.Response, error) {
	f := s.data
	returnStart, returnEnd := paginated(opts.Page, opts.Size, len(f.PullRequestChanges[number]))
	return f.PullRequestChanges[number][returnStart:returnEnd], nil, nil
}

func (s *pullService) ListCommits(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Commit, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) ListComments(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Comment, *scm.Response, error) {
	f := s.data
	return append([]*scm.Comment{}, f.PullRequestComments[number]...), nil, nil
}

func (s *pullService) ListLabels(ctx context.Context, repo string, number int, opts *scm.ListOptions) ([]*scm.Label, *scm.Response, error) {
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

func (s *pullService) ListEvents(context.Context, string, int, *scm.ListOptions) ([]*scm.ListedIssueEvent, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) AddLabel(ctx context.Context, repo string, number int, label string) (*scm.Response, error) {
	f := s.data
	pr := s.data.PullRequests[number]
	if pr != nil {
		found := false
		for _, l := range pr.Labels {
			if l.Name == label {
				found = true
				break
			}
		}
		if !found {
			pr.Labels = append(pr.Labels, &scm.Label{
				ID:   int64(len(pr.Labels)),
				Name: label,
			})
		}
	}
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

func (s *pullService) Merge(ctx context.Context, repo string, number int, mergeOpts *scm.PullRequestMergeOptions) (*scm.Response, error) {
	pr, ok := s.data.PullRequests[number]
	if !ok || pr == nil {
		return nil, fmt.Errorf("pull request %d not found", number)
	}
	pr.Merged = true
	pr.State = "closed"
	pr.Closed = true
	pr.Mergeable = false
	return nil, nil
}

func (s *pullService) Update(_ context.Context, fullName string, number int, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	f := s.data
	namespace := ""
	name := ""
	paths := strings.SplitN(fullName, "/", 2)
	if len(paths) > 1 {
		namespace = paths[0]
		name = paths[1]
	}
	answer := &scm.PullRequest{
		Number: number,
		Title:  input.Title,
		Body:   input.Body,
		Base: scm.PullRequestBranch{
			Ref: input.Base,
			Repo: scm.Repository{
				Namespace: namespace,
				Name:      name,
				FullName:  fullName,
			},
		},
		Head: scm.PullRequestBranch{
			Ref: input.Head,
		},
	}
	f.PullRequestsCreated[number] = input
	f.PullRequests[number] = answer
	return answer, nil, nil
}

func (s *pullService) Close(_ context.Context, fullName string, number int) (*scm.Response, error) {
	pr, ok := s.data.PullRequests[number]
	if !ok || pr == nil {
		return nil, fmt.Errorf("pull request %d not found", number)
	}
	pr.State = "closed"
	pr.Closed = true
	return nil, nil
}

func (s *pullService) Reopen(context.Context, string, int) (*scm.Response, error) {
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

func (s *pullService) DeleteComment(ctx context.Context, repo string, number, id int) (*scm.Response, error) {
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

func (s *pullService) EditComment(ctx context.Context, repo string, number, id int, input *scm.CommentInput) (*scm.Comment, *scm.Response, error) {
	return nil, nil, scm.ErrNotSupported
}

func (s *pullService) AssignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
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

func (s *pullService) UnassignIssue(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	panic("implement me")
}

func (s *pullService) RequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) UnrequestReview(ctx context.Context, repo string, number int, logins []string) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) Create(_ context.Context, fullName string, input *scm.PullRequestInput) (*scm.PullRequest, *scm.Response, error) {
	f := s.data

	for i := range f.PullRequests {
		if f.PullRequests[i].Head.Ref == input.Head && f.PullRequests[i].Base.Ref == input.Base && !f.PullRequests[i].Closed && !f.PullRequests[i].Merged {
			return nil, nil, fmt.Errorf("open pull request from branch %s to branch %s exists, cannot open a duplicate", input.Head, input.Base)
		}
	}

	f.PullRequestID++
	namespace := ""
	name := ""
	paths := strings.SplitN(fullName, "/", 2)
	if len(paths) > 1 {
		namespace = paths[0]
		name = paths[1]
	}
	answer := &scm.PullRequest{
		Number: f.PullRequestID,
		Title:  input.Title,
		Body:   input.Body,
		Base: scm.PullRequestBranch{
			Ref: input.Base,
			Repo: scm.Repository{
				Namespace: namespace,
				Name:      name,
				FullName:  fullName,
			},
		},
		Source: input.Head,
		Link:   fmt.Sprintf("https://api.fake.com/pull/%d", f.PullRequestID),
		Head: scm.PullRequestBranch{
			Ref: input.Head,
		},
	}
	f.PullRequestsCreated[f.PullRequestID] = input
	f.PullRequests[f.PullRequestID] = answer
	return answer, nil, nil
}

func (s *pullService) SetMilestone(ctx context.Context, repo string, prID, number int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}

func (s *pullService) ClearMilestone(ctx context.Context, repo string, prID int) (*scm.Response, error) {
	return nil, scm.ErrNotSupported
}
