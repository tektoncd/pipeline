package transport

import (
	"errors"
	"net/http"

	"github.com/jenkins-x/go-scm/scm"
)

// A Credential attaches a resolved token to a request in whichever form the
// provider expects.
type Credential func(r *http.Request, token string)

// Auth is an http.RoundTripper that resolves its credential from a
// scm.TokenSource on every request rather than capturing it, so a token that
// expires part way through the life of a client is replaced without the client
// having to be rebuilt. Both Source and Credential are required.
type Auth struct {
	Base http.RoundTripper

	Source     scm.TokenSource
	Credential Credential
}

// RoundTrip resolves a token and attaches it to the request.
func (t *Auth) RoundTrip(r *http.Request) (*http.Response, error) {
	if t.Source == nil || t.Credential == nil {
		return nil, errors.New("transport: Auth requires both a Source and a Credential")
	}
	token, err := t.Source.Token(r.Context())
	if err != nil {
		return nil, err
	}
	if token == nil || token.Token == "" {
		return t.base().RoundTrip(r)
	}
	r2 := cloneRequest(r)
	t.Credential(r2, token.Token)
	return t.base().RoundTrip(r2)
}

// base returns the base transport. If no base transport
// is configured, the default transport is returned.
func (t *Auth) base() http.RoundTripper {
	if t.Base != nil {
		return t.Base
	}
	return http.DefaultTransport
}

// SchemeCredential returns a Credential that sets an Authorization header with
// the given scheme, such as "Bearer", or the non-standard "token" scheme that
// Gogs and Gitea use.
func SchemeCredential(scheme string) Credential {
	return func(r *http.Request, token string) {
		r.Header.Set("Authorization", scheme+" "+token)
	}
}

// BasicAuthCredential returns a Credential that sends the token as the basic
// auth password. Azure DevOps expects an empty username.
func BasicAuthCredential(username string) Credential {
	return func(r *http.Request, token string) {
		r.SetBasicAuth(username, token)
	}
}

// PrivateTokenCredential sets the Private-Token header that GitLab expects.
func PrivateTokenCredential(r *http.Request, token string) {
	r.Header.Set("Private-Token", token)
}
