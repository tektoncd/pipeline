package github

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/jenkins-x/go-scm/scm"
)

type deploymentService struct {
	client *wrapper
}

type deployment struct {
	Namespace             string
	Name                  string
	FullName              string
	ID                    int         `json:"id"`
	Link                  string      `json:"url"`
	Sha                   string      `json:"sha"`
	Ref                   string      `json:"ref"`
	Task                  string      `json:"task"`
	Description           string      `json:"description"`
	OriginalEnvironment   string      `json:"original_environment"`
	Environment           string      `json:"environment"`
	EnvironmentURL        string      `json:"environment_url"`
	RepositoryLink        string      `json:"repository_url"`
	StatusLink            string      `json:"statuses_url"`
	Author                *user       `json:"creator"`
	Created               time.Time   `json:"created_at"`
	Updated               time.Time   `json:"updated_at"`
	TransientEnvironment  bool        `json:"transient_environment"`
	ProductionEnvironment bool        `json:"production_environment"`
	Payload               interface{} `json:"payload"`
}

type deploymentInput struct {
	Ref                   string   `json:"ref,omitempty"`
	Task                  string   `json:"task,omitempty"`
	Payload               string   `json:"payload,omitempty"`
	Environment           string   `json:"environment,omitempty"`
	Description           string   `json:"description,omitempty"`
	RequiredContexts      []string `json:"required_contexts,omitempty"`
	AutoMerge             bool     `json:"auto_merge"`
	TransientEnvironment  bool     `json:"transient_environment"`
	ProductionEnvironment bool     `json:"production_environment"`
}

type deploymentStatus struct {
	ID              int       `json:"id"`
	State           string    `json:"state"`
	Author          *user     `json:"creator"`
	Description     string    `json:"description"`
	Environment     string    `json:"environment"`
	DeploymentLink  string    `json:"deployment_url"`
	EnvironmentLink string    `json:"environment_url"`
	LogLink         string    `json:"log_url"`
	RepositoryLink  string    `json:"repository_url"`
	TargetLink      string    `json:"target_url"`
	Created         time.Time `json:"created_at"`
	Updated         time.Time `json:"updated_at"`
}

type deploymentStatusInput struct {
	State           string `json:"state"`
	TargetLink      string `json:"target_url"`
	LogLink         string `json:"log_url"`
	Description     string `json:"description"`
	Environment     string `json:"environment"`
	EnvironmentLink string `json:"environment_url"`
	AutoInactive    bool   `json:"auto_inactive"`
}

func (s *deploymentService) Find(ctx context.Context, repoFullName, deploymentID string) (*scm.Deployment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments/%s", repoFullName, deploymentID)
	out := new(deployment)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertDeployment(out, repoFullName), res, wrapError(res, err)
}

func (s *deploymentService) List(ctx context.Context, repoFullName string, opts *scm.ListOptions) ([]*scm.Deployment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments?%s", repoFullName, encodeListOptions(opts))
	out := []*deployment{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertDeploymentList(out, repoFullName), res, wrapError(res, err)
}

func (s *deploymentService) Create(ctx context.Context, repoFullName string, deploymentInput *scm.DeploymentInput) (*scm.Deployment, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments", repoFullName)
	in := convertToDeploymentInput(deploymentInput)
	out := new(deployment)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertDeployment(out, repoFullName), res, wrapError(res, err)
}

func (s *deploymentService) Delete(ctx context.Context, repoFullName, deploymentID string) (*scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments/%s", repoFullName, deploymentID)
	return s.client.do(ctx, "DELETE", path, nil, nil)
}

func (s *deploymentService) FindStatus(ctx context.Context, repoFullName, deploymentID, statusID string) (*scm.DeploymentStatus, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments/%s/statuses/%s", repoFullName, deploymentID, statusID)
	out := new(deploymentStatus)
	res, err := s.client.do(ctx, "GET", path, nil, out)
	return convertDeploymentStatus(out), res, wrapError(res, err)
}

func (s *deploymentService) ListStatus(ctx context.Context, repoFullName, deploymentID string, opts *scm.ListOptions) ([]*scm.DeploymentStatus, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments/%s/statuses?%s", repoFullName, deploymentID, encodeListOptions(opts))
	out := []*deploymentStatus{}
	res, err := s.client.do(ctx, "GET", path, nil, &out)
	return convertDeploymentStatusList(out), res, wrapError(res, err)
}

func (s *deploymentService) CreateStatus(ctx context.Context, repoFullName, deploymentID string, deploymentStatusInput *scm.DeploymentStatusInput) (*scm.DeploymentStatus, *scm.Response, error) {
	path := fmt.Sprintf("repos/%s/deployments/%s/statuses", repoFullName, deploymentID)
	in := convertToDeploymentStatusInput(deploymentStatusInput)
	out := new(deploymentStatus)
	res, err := s.client.do(ctx, "POST", path, in, out)
	return convertDeploymentStatus(out), res, wrapError(res, err)
}

func wrapError(res *scm.Response, err error) error {
	if res == nil {
		return err
	}
	data, err2 := io.ReadAll(res.Body)
	if err2 != nil {
		return errors.Wrapf(err, "http status %d", res.Status)
	}
	return errors.Wrapf(err, "http status %d mesage %s", res.Status, string(data))
}

func convertDeploymentList(out []*deployment, fullName string) []*scm.Deployment {
	answer := []*scm.Deployment{}
	for _, o := range out {
		answer = append(answer, convertDeployment(o, fullName))
	}
	return answer
}

func convertDeploymentStatusList(out []*deploymentStatus) []*scm.DeploymentStatus {
	answer := []*scm.DeploymentStatus{}
	for _, o := range out {
		answer = append(answer, convertDeploymentStatus(o))
	}
	return answer
}

func convertToDeploymentInput(from *scm.DeploymentInput) *deploymentInput {
	return &deploymentInput{
		Ref:                   from.Ref,
		Task:                  from.Task,
		Payload:               from.Payload,
		Environment:           from.Environment,
		Description:           from.Description,
		RequiredContexts:      from.RequiredContexts,
		AutoMerge:             from.AutoMerge,
		TransientEnvironment:  from.TransientEnvironment,
		ProductionEnvironment: from.ProductionEnvironment,
	}
}

func convertDeployment(from *deployment, fullName string) *scm.Deployment {
	dst := &scm.Deployment{
		ID:                    strconv.Itoa(from.ID),
		Link:                  from.Link,
		Sha:                   from.Sha,
		Ref:                   from.Ref,
		FullName:              fullName,
		Task:                  from.Task,
		Description:           from.Description,
		OriginalEnvironment:   from.OriginalEnvironment,
		Environment:           from.Environment,
		RepositoryLink:        from.RepositoryLink,
		StatusLink:            from.StatusLink,
		Author:                convertUser(from.Author),
		Created:               from.Created,
		Updated:               from.Updated,
		TransientEnvironment:  from.TransientEnvironment,
		ProductionEnvironment: from.ProductionEnvironment,
		Payload:               from.Payload,
	}
	names := strings.Split(fullName, "/")
	if len(names) > 1 {
		dst.Namespace = names[0]
		dst.Name = names[1]
	}
	return dst
}

func convertDeploymentStatus(from *deploymentStatus) *scm.DeploymentStatus {
	return &scm.DeploymentStatus{
		ID:              strconv.Itoa(from.ID),
		State:           from.State,
		Author:          convertUser(from.Author),
		Description:     from.Description,
		Environment:     from.Environment,
		DeploymentLink:  from.DeploymentLink,
		EnvironmentLink: from.EnvironmentLink,
		LogLink:         from.LogLink,
		RepositoryLink:  from.RepositoryLink,
		TargetLink:      from.TargetLink,
		Created:         from.Created,
		Updated:         from.Updated,
	}
}

func convertToDeploymentStatusInput(from *scm.DeploymentStatusInput) *deploymentStatusInput {
	return &deploymentStatusInput{
		State:           from.State,
		TargetLink:      from.TargetLink,
		LogLink:         from.LogLink,
		Description:     from.Description,
		Environment:     from.Environment,
		EnvironmentLink: from.EnvironmentLink,
		AutoInactive:    from.AutoInactive,
	}
}
