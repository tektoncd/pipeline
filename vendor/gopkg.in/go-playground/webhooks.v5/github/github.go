package github

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
)

// parse errors
var (
	ErrEventNotSpecifiedToParse  = errors.New("no Event specified to parse")
	ErrInvalidHTTPMethod         = errors.New("invalid HTTP Method")
	ErrMissingGithubEventHeader  = errors.New("missing X-GitHub-Event Header")
	ErrMissingHubSignatureHeader = errors.New("missing X-Hub-Signature Header")
	ErrEventNotFound             = errors.New("event not defined to be parsed")
	ErrParsingPayload            = errors.New("error parsing payload")
	ErrHMACVerificationFailed    = errors.New("HMAC verification failed")
)

// Event defines a GitHub hook event type
type Event string

// GitHub hook types
const (
	CheckRunEvent                 Event = "check_run"
	CheckSuiteEvent               Event = "check_suite"
	CommitCommentEvent            Event = "commit_comment"
	CreateEvent                   Event = "create"
	DeleteEvent                   Event = "delete"
	DeploymentEvent               Event = "deployment"
	DeploymentStatusEvent         Event = "deployment_status"
	ForkEvent                     Event = "fork"
	GollumEvent                   Event = "gollum"
	InstallationEvent             Event = "installation"
	InstallationRepositoriesEvent Event = "installation_repositories"
	IntegrationInstallationEvent  Event = "integration_installation"
	IssueCommentEvent             Event = "issue_comment"
	IssuesEvent                   Event = "issues"
	LabelEvent                    Event = "label"
	MemberEvent                   Event = "member"
	MembershipEvent               Event = "membership"
	MilestoneEvent                Event = "milestone"
	OrganizationEvent             Event = "organization"
	OrgBlockEvent                 Event = "org_block"
	PageBuildEvent                Event = "page_build"
	PingEvent                     Event = "ping"
	ProjectCardEvent              Event = "project_card"
	ProjectColumnEvent            Event = "project_column"
	ProjectEvent                  Event = "project"
	PublicEvent                   Event = "public"
	PullRequestEvent              Event = "pull_request"
	PullRequestReviewEvent        Event = "pull_request_review"
	PullRequestReviewCommentEvent Event = "pull_request_review_comment"
	PushEvent                     Event = "push"
	ReleaseEvent                  Event = "release"
	RepositoryEvent               Event = "repository"
	StatusEvent                   Event = "status"
	TeamEvent                     Event = "team"
	TeamAddEvent                  Event = "team_add"
	WatchEvent                    Event = "watch"
)

// EventSubtype defines a GitHub Hook Event subtype
type EventSubtype string

// GitHub hook event subtypes
const (
	NoSubtype     EventSubtype = ""
	BranchSubtype EventSubtype = "branch"
	TagSubtype    EventSubtype = "tag"
	PullSubtype   EventSubtype = "pull"
	IssueSubtype  EventSubtype = "issues"
)

// Option is a configuration option for the webhook
type Option func(*Webhook) error

// Options is a namespace var for configuration options
var Options = WebhookOptions{}

// WebhookOptions is a namespace for configuration option methods
type WebhookOptions struct{}

// Secret registers the GitHub secret
func (WebhookOptions) Secret(secret string) Option {
	return func(hook *Webhook) error {
		hook.secret = secret
		return nil
	}
}

// Webhook instance contains all methods needed to process events
type Webhook struct {
	secret string
}

// New creates and returns a WebHook instance denoted by the Provider type
func New(options ...Option) (*Webhook, error) {
	hook := new(Webhook)
	for _, opt := range options {
		if err := opt(hook); err != nil {
			return nil, errors.New("Error applying Option")
		}
	}
	return hook, nil
}

// Parse verifies and parses the events specified and returns the payload object or an error
func (hook Webhook) Parse(r *http.Request, events ...Event) (interface{}, error) {
	defer func() {
		_, _ = io.Copy(ioutil.Discard, r.Body)
		_ = r.Body.Close()
	}()

	if len(events) == 0 {
		return nil, ErrEventNotSpecifiedToParse
	}
	if r.Method != http.MethodPost {
		return nil, ErrInvalidHTTPMethod
	}

	event := r.Header.Get("X-GitHub-Event")
	if event == "" {
		return nil, ErrMissingGithubEventHeader
	}
	gitHubEvent := Event(event)

	var found bool
	for _, evt := range events {
		if evt == gitHubEvent {
			found = true
			break
		}
	}
	// event not defined to be parsed
	if !found {
		return nil, ErrEventNotFound
	}

	payload, err := ioutil.ReadAll(r.Body)
	if err != nil || len(payload) == 0 {
		return nil, ErrParsingPayload
	}

	// If we have a Secret set, we should check the MAC
	if len(hook.secret) > 0 {
		signature := r.Header.Get("X-Hub-Signature")
		if len(signature) == 0 {
			return nil, ErrMissingHubSignatureHeader
		}
		mac := hmac.New(sha1.New, []byte(hook.secret))
		_, _ = mac.Write(payload)
		expectedMAC := hex.EncodeToString(mac.Sum(nil))

		if !hmac.Equal([]byte(signature[5:]), []byte(expectedMAC)) {
			return nil, ErrHMACVerificationFailed
		}
	}

	switch gitHubEvent {
	case CheckRunEvent:
		var pl CheckRunPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case CheckSuiteEvent:
		var pl CheckSuitePayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case CommitCommentEvent:
		var pl CommitCommentPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case CreateEvent:
		var pl CreatePayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case DeleteEvent:
		var pl DeletePayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case DeploymentEvent:
		var pl DeploymentPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case DeploymentStatusEvent:
		var pl DeploymentStatusPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case ForkEvent:
		var pl ForkPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case GollumEvent:
		var pl GollumPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case InstallationEvent, IntegrationInstallationEvent:
		var pl InstallationPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case InstallationRepositoriesEvent:
		var pl InstallationRepositoriesPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case IssueCommentEvent:
		var pl IssueCommentPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case IssuesEvent:
		var pl IssuesPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case LabelEvent:
		var pl LabelPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case MemberEvent:
		var pl MemberPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case MembershipEvent:
		var pl MembershipPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case MilestoneEvent:
		var pl MilestonePayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case OrganizationEvent:
		var pl OrganizationPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case OrgBlockEvent:
		var pl OrgBlockPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PageBuildEvent:
		var pl PageBuildPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PingEvent:
		var pl PingPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case ProjectCardEvent:
		var pl ProjectCardPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case ProjectColumnEvent:
		var pl ProjectColumnPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case ProjectEvent:
		var pl ProjectPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PublicEvent:
		var pl PublicPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestEvent:
		var pl PullRequestPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestReviewEvent:
		var pl PullRequestReviewPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PullRequestReviewCommentEvent:
		var pl PullRequestReviewCommentPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case PushEvent:
		var pl PushPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case ReleaseEvent:
		var pl ReleasePayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case RepositoryEvent:
		var pl RepositoryPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case StatusEvent:
		var pl StatusPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case TeamEvent:
		var pl TeamPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case TeamAddEvent:
		var pl TeamAddPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	case WatchEvent:
		var pl WatchPayload
		err = json.Unmarshal([]byte(payload), &pl)
		return pl, err
	default:
		return nil, fmt.Errorf("unknown event %s", gitHubEvent)
	}
}
