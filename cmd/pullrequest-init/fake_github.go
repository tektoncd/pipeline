package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/go-github/github"
	"github.com/gorilla/mux"
)

const (
	// ErrorKeyword is a magic const used to denote PRs/Comments that should
	// return errors to the client to simulate issues communicating with GitHub.
	ErrorKeyword = "~~ERROR~~"
)

// key defines keys for associating data to PRs/issues in the fake server.
type key struct {
	owner string
	repo  string
	id    int64
}

// FakeGitHub is a fake GitHub server for use in tests.
type FakeGitHub struct {
	*mux.Router

	pr map[key]*github.PullRequest
	// GitHub references comments in 2 ways:
	// 1) List by issue (PR) ID.
	// 2) Get by comment ID.
	// We need to store references to both to emulate the API properly.
	prComments map[key][]*github.IssueComment
	comments   map[key]*github.IssueComment
	status     map[statusKey]map[string]*github.RepoStatus
}

// NewFakeGitHub returns a new FakeGitHub.
func NewFakeGitHub() *FakeGitHub {
	s := &FakeGitHub{
		Router:     mux.NewRouter(),
		pr:         make(map[key]*github.PullRequest),
		prComments: make(map[key][]*github.IssueComment),
		comments:   make(map[key]*github.IssueComment),
		status:     make(map[statusKey]map[string]*github.RepoStatus),
	}
	s.HandleFunc("/repos/{owner}/{repo}/pulls/{number}", s.getPullRequest).Methods(http.MethodGet)
	s.HandleFunc("/repos/{owner}/{repo}/issues/{number}/comments", s.getComments).Methods(http.MethodGet)
	s.HandleFunc("/repos/{owner}/{repo}/issues/{number}/comments", s.createComment).Methods(http.MethodPost)
	s.HandleFunc("/repos/{owner}/{repo}/issues/comments/{number}", s.updateComment).Methods(http.MethodPatch)
	s.HandleFunc("/repos/{owner}/{repo}/issues/comments/{number}", s.deleteComment).Methods(http.MethodDelete)
	s.HandleFunc("/repos/{owner}/{repo}/issues/{number}/labels", s.updateLabels).Methods(http.MethodPut)
	s.HandleFunc("/repos/{owner}/{repo}/statuses/{revision}", s.createStatus).Methods(http.MethodPost)
	s.HandleFunc("/repos/{owner}/{repo}/commits/{revision}/status", s.getStatuses).Methods(http.MethodGet)

	return s
}

func prKey(r *http.Request) (key, error) {
	pr, err := strconv.ParseInt(mux.Vars(r)["number"], 10, 64)
	if err != nil {
		return key{}, err
	}
	return key{
		owner: mux.Vars(r)["owner"],
		repo:  mux.Vars(r)["repo"],
		id:    pr,
	}, nil
}

// AddPullRequest adds the given pull request to the fake GitHub server.
// This is done as a convenience over implementing PullReqests.Create in the
// GitHub server since that method takes in a different type (NewPullRequest).
func (g *FakeGitHub) AddPullRequest(pr *github.PullRequest) {
	key := key{
		owner: pr.GetBase().GetUser().GetLogin(),
		repo:  pr.GetBase().GetRepo().GetName(),
		id:    pr.GetID(),
	}
	g.pr[key] = pr
}

func (g *FakeGitHub) getPullRequest(w http.ResponseWriter, r *http.Request) {
	key, err := prKey(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pr, ok := g.pr[key]
	if !ok {
		http.Error(w, fmt.Sprintf("%v not found", key), http.StatusNotFound)
		return
	}

	if err := json.NewEncoder(w).Encode(pr); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (g *FakeGitHub) getComments(w http.ResponseWriter, r *http.Request) {
	key, err := prKey(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	comments, ok := g.prComments[key]
	if !ok {
		comments = []*github.IssueComment{}
	}
	if err := json.NewEncoder(w).Encode(comments); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (g *FakeGitHub) createComment(w http.ResponseWriter, r *http.Request) {
	prKey, err := prKey(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	c := new(github.IssueComment)
	if err := json.NewDecoder(r.Body).Decode(c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.Contains(c.GetBody(), ErrorKeyword) {
		http.Error(w, "intentional error", http.StatusInternalServerError)
		return
	}

	c.ID = github.Int64(int64(len(g.comments) + 1))

	if _, ok := g.prComments[prKey]; !ok {
		g.prComments[prKey] = []*github.IssueComment{}
	}
	g.prComments[prKey] = append(g.prComments[prKey], c)

	commentKey := key{
		owner: prKey.owner,
		repo:  prKey.repo,
		id:    c.GetID(),
	}
	g.comments[commentKey] = c

	if err := json.NewEncoder(w).Encode(c); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (g *FakeGitHub) updateComment(w http.ResponseWriter, r *http.Request) {
	key, err := prKey(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	update := new(github.IssueComment)
	if err := json.NewDecoder(r.Body).Decode(update); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if strings.Contains(update.GetBody(), ErrorKeyword) {
		http.Error(w, "intentional error", http.StatusInternalServerError)
		return
	}

	existing, ok := g.comments[key]
	if !ok {
		http.Error(w, fmt.Sprintf("comment %+v not found", key), http.StatusNotFound)
		return
	}

	*existing = *update
	w.WriteHeader(http.StatusOK)
}

func (g *FakeGitHub) deleteComment(w http.ResponseWriter, r *http.Request) {
	key, err := prKey(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, ok := g.comments[key]; !ok {
		http.Error(w, fmt.Sprintf("comment %+v not found", key), http.StatusNotFound)
		return
	}

	// Remove comment from PR storage. Not particularly efficient, but we don't
	// generally expect this to be used for large number of comments in unit
	// tests.
	for k := range g.prComments {
		for i, c := range g.prComments[k] {
			if c.GetID() == key.id {
				g.prComments[k] = append(g.prComments[k][:i], g.prComments[k][i+1:]...)
				break
			}
		}
	}
	delete(g.comments, key)

	w.WriteHeader(http.StatusOK)
}

func (g *FakeGitHub) updateLabels(w http.ResponseWriter, r *http.Request) {
	key, err := prKey(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
	}

	pr, ok := g.pr[key]
	if !ok {
		http.Error(w, "pull request not found", http.StatusNotFound)
		return
	}

	payload := []string{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pr.Labels = make([]*github.Label, 0, len(payload))
	for _, l := range payload {
		pr.Labels = append(pr.Labels, &github.Label{
			Name: github.String(l),
		})
	}

	w.WriteHeader(http.StatusOK)
}

type statusKey struct {
	owner    string
	repo     string
	revision string
}

func getStatusKey(r *http.Request) statusKey {
	return statusKey{
		owner:    mux.Vars(r)["owner"],
		repo:     mux.Vars(r)["repo"],
		revision: mux.Vars(r)["revision"],
	}
}

func (g *FakeGitHub) createStatus(w http.ResponseWriter, r *http.Request) {
	k := getStatusKey(r)

	rs := new(github.RepoStatus)
	if err := json.NewDecoder(r.Body).Decode(rs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if _, ok := g.status[k]; !ok {
		g.status[k] = make(map[string]*github.RepoStatus)
	}
	g.status[k][rs.GetContext()] = rs
}

func (g *FakeGitHub) getStatuses(w http.ResponseWriter, r *http.Request) {
	k := getStatusKey(r)

	s := make([]github.RepoStatus, 0, len(g.status[k]))
	for _, v := range g.status[k] {
		s = append(s, *v)
	}

	cs := &github.CombinedStatus{
		TotalCount: github.Int(len(s)),
		Statuses:   s,
	}
	if err := json.NewEncoder(w).Encode(cs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
