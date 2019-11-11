package fake

import "github.com/jenkins-x/go-scm/scm"

type Data struct {
	Issues                    map[int][]*scm.Issue
	OrgMembers                map[string][]string
	Collaborators             []string
	IssueComments             map[int][]*scm.Comment
	IssueCommentID            int
	PullRequests              map[int]*scm.PullRequest
	PullRequestChanges        map[int][]*scm.Change
	PullRequestComments       map[int][]*scm.Comment
	PullRequestLabelsAdded    []string
	PullRequestLabelsRemoved  []string
	PullRequestLabelsExisting []string
	ReviewID                  int
	Reviews                   map[int][]*scm.Review
	Statuses                  map[string][]*scm.Status
	IssueEvents               map[int][]*scm.ListedIssueEvent
	Commits                   map[string]*scm.Commit
	TestRef                   string

	//All Labels That Exist In The Repo
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
}

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
	}
}
