package fake

import "github.com/jenkins-x/go-scm/scm"

// Data is used to store/represent test data for the fake client
type Data struct {
	Issues                     map[int][]*scm.Issue
	OrgMembers                 map[string][]string
	Collaborators              []string
	IssueComments              map[int][]*scm.Comment
	IssueCommentID             int
	PullRequests               map[int]*scm.PullRequest
	PullRequestChanges         map[int][]*scm.Change
	PullRequestComments        map[int][]*scm.Comment
	PullRequestCommentsAdded   []string
	PullRequestCommentsDeleted []string
	PullRequestLabelsAdded     []string
	PullRequestLabelsRemoved   []string
	PullRequestLabelsExisting  []string
	ReviewID                   int
	Reviews                    map[int][]*scm.Review
	Statuses                   map[string][]*scm.Status
	IssueEvents                map[int][]*scm.ListedIssueEvent
	Commits                    map[string]*scm.Commit
	TestRef                    string
	PullRequestsCreated        map[int]*scm.PullRequestInput
	PullRequestID              int
	CreateRepositories         []*scm.RepositoryInput
	Organizations              []*scm.Organization
	Repositories               []*scm.Repository
	CurrentUser                scm.User
	Users                      []*scm.User
	Hooks                      map[string][]*scm.Hook
	Releases                   map[string]map[int]*scm.Release
	Deployments                map[string][]*scm.Deployment
	DeploymentStatus           map[string][]*scm.DeploymentStatus

	// All Labels That Exist In The Repo
	RepoLabelsExisting []string
	// org/repo#number:label
	IssueLabelsAdded    []string
	IssueLabelsExisting []string
	IssueLabelsRemoved  []string

	// org/repo#number:body
	IssueCommentsAdded []string
	// org/repo#issuecommentid
	IssueCommentsDeleted []string

	// org/repo#issuecommentid:reaction
	IssueReactionsAdded   []string
	CommentReactionsAdded []string

	// org/repo#number:assignee
	AssigneesAdded []string

	// org/repo#number:milestone (represents the milestone for a specific issue)
	Milestone    int
	MilestoneMap map[string]int

	// list of commits for each PR
	// org/repo#number:[]commit
	CommitMap map[string][]scm.Commit

	// Fake remote git storage. File name are keys
	// and values map SHA to content
	RemoteFiles map[string]map[string]string

	// A list of refs that got deleted via DeleteRef
	RefsDeleted []DeletedRef

	UserPermissions map[string]map[string]string

	// Invitations the current pending invitations
	Invitations []*scm.Invitation

	// ContentDir the directory used to implement the Content service to access files and directories
	ContentDir string
}

// DeletedRef represents a ref that has been deleted
type DeletedRef struct {
	Org, Repo, Ref string
}

// NewData create a new set of fake data
func NewData() *Data {
	return &Data{
		Issues:                    map[int][]*scm.Issue{},
		OrgMembers:                map[string][]string{},
		Collaborators:             []string{},
		IssueComments:             map[int][]*scm.Comment{},
		PullRequests:              map[int]*scm.PullRequest{},
		PullRequestChanges:        map[int][]*scm.Change{},
		PullRequestComments:       map[int][]*scm.Comment{},
		PullRequestLabelsAdded:    []string{},
		PullRequestLabelsRemoved:  []string{},
		PullRequestLabelsExisting: []string{},
		PullRequestsCreated:       map[int]*scm.PullRequestInput{},
		Reviews:                   map[int][]*scm.Review{},
		Statuses:                  map[string][]*scm.Status{},
		IssueEvents:               map[int][]*scm.ListedIssueEvent{},
		Commits:                   map[string]*scm.Commit{},
		MilestoneMap:              map[string]int{},
		CommitMap:                 map[string][]scm.Commit{},
		RemoteFiles:               map[string]map[string]string{},
		TestRef:                   "abcde",
		IssueLabelsAdded:          []string{},
		IssueLabelsExisting:       []string{},
		IssueLabelsRemoved:        []string{},
		IssueCommentsAdded:        []string{},
		IssueCommentsDeleted:      []string{},
		IssueReactionsAdded:       []string{},
		CommentReactionsAdded:     []string{},
		AssigneesAdded:            []string{},
		UserPermissions:           map[string]map[string]string{},
		Hooks:                     map[string][]*scm.Hook{},
		Deployments:               map[string][]*scm.Deployment{},
		DeploymentStatus:          map[string][]*scm.DeploymentStatus{},
	}
}
