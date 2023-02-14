package factory

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/jenkins-x/go-scm/scm/driver/azure"
	"github.com/pkg/errors"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/jenkins-x/go-scm/scm/driver/bitbucket"
	"github.com/jenkins-x/go-scm/scm/driver/fake"
	"github.com/jenkins-x/go-scm/scm/driver/gitea"
	"github.com/jenkins-x/go-scm/scm/driver/github"
	"github.com/jenkins-x/go-scm/scm/driver/gitlab"
	"github.com/jenkins-x/go-scm/scm/driver/gogs"
	"github.com/jenkins-x/go-scm/scm/driver/stash"
	"github.com/jenkins-x/go-scm/scm/transport"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// ErrMissingGitServerURL the error returned if you use a git driver that needs a git server URL
var ErrMissingGitServerURL = fmt.Errorf("No git serverURL was specified")

// DefaultIdentifier is the default driver identifier used by FromRepoURL.
var DefaultIdentifier = NewDriverIdentifier()

// ClientOptionFunc is a function taking a client as its argument
type ClientOptionFunc func(*scm.Client)

type AuthOptions struct {
	oauthToken   string
	clientID     string
	clientSecret string
}

// SetUsername allows the username to be set
func SetUsername(username string) ClientOptionFunc {
	return func(client *scm.Client) {
		if username != "" {
			client.Username = username
		}
	}
}

// NewClientWithBasicAuth creates a new client for a given driver, serverURL and basic auth
func NewClientWithBasicAuth(driver, serverURL, user, password string, opts ...ClientOptionFunc) (*scm.Client, error) {
	if driver == "" {
		driver = "github"
	}
	var client *scm.Client
	var err error

	switch driver {
	case "gitea":
		if serverURL == "" {
			return nil, ErrMissingGitServerURL
		}
		client, err = gitea.NewWithBasicAuth(serverURL, user, password)
	default:
		return nil, fmt.Errorf("Unsupported $GIT_KIND value: %s", driver)
	}
	if err != nil {
		return client, err
	}
	for _, o := range opts {
		o(client)
	}
	return client, err
}

// NewClient creates a new client for a given driver, serverURL and OAuth token
func NewClient(driver, serverURL, oauthToken string, opts ...ClientOptionFunc) (*scm.Client, error) {
	authOptions := &AuthOptions{
		oauthToken: oauthToken,
	}
	return newClient(driver, serverURL, authOptions, opts...)
}

func newClient(driver, serverURL string, authOptions *AuthOptions, opts ...ClientOptionFunc) (*scm.Client, error) {
	oauthToken := authOptions.oauthToken
	if driver == "" {
		driver = "github"
	}
	var client *scm.Client
	var err error

	switch driver {
	case "azure":
		client = azure.NewDefault()
	case "bitbucket", "bitbucketcloud":
		if serverURL != "" {
			client, err = bitbucket.New(ensureBBCEndpoint(serverURL))
		} else {
			client = bitbucket.NewDefault()
		}
	case "fake", "fakegit":
		client, _ = fake.NewDefault()
	case "gitea":
		if serverURL == "" {
			return nil, ErrMissingGitServerURL
		}
		client, err = gitea.NewWithToken(serverURL, oauthToken)
	case "github":
		if serverURL != "" {
			client, err = github.New(ensureGHEEndpoint(serverURL))
		} else {
			client = github.NewDefault()
		}
	case "gitlab":
		if serverURL != "" {
			client, err = gitlab.New(serverURL)
		} else {
			client = gitlab.NewDefault()
		}
	case "gogs":
		if serverURL == "" {
			return nil, ErrMissingGitServerURL
		}
		client, err = gogs.New(serverURL)
	case "stash", "bitbucketserver":
		if serverURL == "" {
			return nil, ErrMissingGitServerURL
		}
		client, err = stash.New(serverURL)
	default:
		return nil, fmt.Errorf("Unsupported $GIT_KIND value: %s", driver)
	}
	if err != nil {
		return client, err
	}
	if oauthToken != "" {
		switch driver {
		case "azure":
			client.Client = &http.Client{
				Transport: &transport.Custom{
					Before: func(r *http.Request) {
						encoded := base64.StdEncoding.EncodeToString([]byte(":" + oauthToken))
						r.Header.Set("Authorization", fmt.Sprintf("Basic %s", encoded))
					},
				},
			}
		case "gitea":
			client.Client = &http.Client{
				Transport: &transport.Authorization{
					Scheme:      "token",
					Credentials: oauthToken,
				},
			}
		case "gitlab":
			client.Client = &http.Client{
				Transport: &transport.PrivateToken{
					Token: oauthToken,
				},
			}
		case "bitbucketcloud":
			// lets process any options now so that we can populate the username
			for _, o := range opts {
				o(client)
			}
			if client.Username == "" {
				return nil, errors.Errorf("no username supplied")
			}
			if authOptions.clientID != "" && authOptions.clientSecret != "" {
				config := clientcredentials.Config{
					ClientID:     authOptions.clientID,
					ClientSecret: authOptions.clientSecret,
					TokenURL:     "https://bitbucket.org/site/oauth2/access_token",
				}
				client.Client = config.Client(context.Background())
				return client, nil
			}
			// BB App Password / PAT
			client.Client = &http.Client{
				Transport: &transport.BasicAuth{
					Username: client.Username,
					Password: oauthToken,
				},
			}
			return client, nil
		default:
			ts := oauth2.StaticTokenSource(
				&oauth2.Token{AccessToken: oauthToken},
			)
			client.Client = oauth2.NewClient(context.Background(), ts)
		}
	}
	for _, o := range opts {
		o(client)
	}
	return client, err
}

// NewClientFromEnvironment creates a new client using environment variables $GIT_KIND, $GIT_SERVER, $GIT_TOKEN
// defaulting to github if no $GIT_KIND or $GIT_SERVER
func NewClientFromEnvironment() (*scm.Client, error) {
	if repoURL := os.Getenv("GIT_REPO_URL"); repoURL != "" {
		return FromRepoURL(repoURL)
	}
	driver := os.Getenv("GIT_KIND")
	serverURL := os.Getenv("GIT_SERVER")
	oauthToken := os.Getenv("GIT_TOKEN")
	username := os.Getenv("GIT_USER")
	if username == "" {
		username = os.Getenv("GIT_USERNAME")
	}

	if oauthToken == "" {
		return nil, fmt.Errorf("No Git OAuth token specified for $GIT_TOKEN")
	}

	authOptions := &AuthOptions{
		oauthToken: oauthToken,
	}

	clientID := os.Getenv("BB_OAUTH_CLIENT_ID")
	clientSecret := os.Getenv("BB_OAUTH_CLIENT_SECRET")
	authOptions.clientID = clientID
	authOptions.clientSecret = clientSecret

	client, err := newClient(driver, serverURL, authOptions, SetUsername(username))
	if driver == "" {
		driver = client.Driver.String()
	}
	fmt.Printf("using driver: %s and serverURL: %s\n", driver, serverURL)
	return client, err
}

// FromRepoURL parses a URL of the form https://:authtoken@host/ and attempts to
// determine the driver and creates a client to authenticate to the endpoint.
func FromRepoURL(repoURL string) (*scm.Client, error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return nil, err
	}
	auth := ""
	if password, ok := u.User.Password(); ok {
		auth = password
	}

	driver, err := DefaultIdentifier.Identify(u.Host)
	if err != nil {
		return nil, err
	}
	u.Path = "/"
	u.User = nil
	return NewClient(driver, u.String(), auth)
}

// ensureGHEEndpoint lets ensure we have the /api/v3 suffix on the URL
func ensureGHEEndpoint(u string) string {
	if strings.HasPrefix(u, "https://github.com") || strings.HasPrefix(u, "http://github.com") {
		return "https://api.github.com"
	}
	// lets ensure we use the API endpoint to login
	if !strings.Contains(u, "/api/") {
		u = scm.URLJoin(u, "/api/v3")
	}
	return u
}

// ensureBBCEndpoint lets ensure we have the /api/v3 suffix on the URL
func ensureBBCEndpoint(u string) string {
	if strings.HasPrefix(u, "https://bitbucket.org") || strings.HasPrefix(u, "http://bitbucket.org") {
		return "https://api.bitbucket.org"
	}
	return u
}

// Client creates a new client with the given HTTP client
func Client(httpClient *http.Client) ClientOptionFunc {
	return func(c *scm.Client) {
		c.Client = httpClient
	}
}

// NewWebHookService creates a new instance of the webhook service without the rest of the client
func NewWebHookService(driver string) (scm.WebhookService, error) {
	if driver == "" {
		driver = "github"
	}
	var service scm.WebhookService
	switch driver {
	case "bitbucket", "bitbucketcloud":
		service = bitbucket.NewWebHookService()
	case "fake", "fakegit":
		// TODO: support fake
	case "gitea":
		service = gitea.NewWebHookService()
	case "github":
		service = github.NewWebHookService()
	case "gitlab":
		service = gitlab.NewWebHookService()
	case "gogs":
		service = gogs.NewWebHookService()
	case "stash", "bitbucketserver":
		service = stash.NewWebHookService()
	default:
		return nil, fmt.Errorf("Unsupported GIT_KIND value: %s", driver)
	}

	return service, nil
}
