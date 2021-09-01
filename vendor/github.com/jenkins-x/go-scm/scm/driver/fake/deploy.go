package fake

import (
	"context"
	"strconv"
	"time"

	"github.com/jenkins-x/go-scm/scm"
)

type deploymentService struct {
	client *wrapper
	data   *Data
}

func (s *deploymentService) Find(ctx context.Context, repoFullName string, deploymentID string) (*scm.Deployment, *scm.Response, error) {
	for _, d := range s.data.Deployments[repoFullName] {
		if d.ID == deploymentID {
			return d, nil, nil
		}
	}
	return nil, nil, scm.ErrNotFound
}

func (s *deploymentService) List(ctx context.Context, repoFullName string, opts scm.ListOptions) ([]*scm.Deployment, *scm.Response, error) {
	return s.data.Deployments[repoFullName], nil, nil
}

func (s *deploymentService) Create(ctx context.Context, repoFullName string, input *scm.DeploymentInput) (*scm.Deployment, *scm.Response, error) {
	deployments := s.data.Deployments[repoFullName]
	owner, name := scm.Split(repoFullName)

	d := &scm.Deployment{
		ID:                    "deployment-" + strconv.Itoa(len(deployments)+1),
		Namespace:             owner,
		Name:                  name,
		Link:                  "",
		Sha:                   "",
		Ref:                   input.Ref,
		Task:                  input.Task,
		FullName:              "",
		Description:           input.Description,
		OriginalEnvironment:   input.Environment,
		Environment:           input.Environment,
		RepositoryLink:        "",
		StatusLink:            "",
		Author:                nil,
		Created:               time.Time{},
		Updated:               time.Time{},
		TransientEnvironment:  input.TransientEnvironment,
		ProductionEnvironment: input.ProductionEnvironment,
		Payload:               input.Payload,
	}

	s.data.Deployments[repoFullName] = append(deployments, d)
	return d, nil, nil
}

func (s *deploymentService) Delete(ctx context.Context, repoFullName string, deploymentID string) (*scm.Response, error) {
	deployments := s.data.Deployments[repoFullName]
	for i, d := range deployments {
		if d.ID == deploymentID {
			result := deployments[0:i]
			if i+1 < len(deployments) {
				result = append(result, deployments[i+1:]...)
			}
			s.data.Deployments[repoFullName] = result
			return nil, nil
		}
	}
	return nil, scm.ErrNotFound
}

func (s *deploymentService) FindStatus(ctx context.Context, repoFullName string, deploymentID string, statusID string) (*scm.DeploymentStatus, *scm.Response, error) {
	key := scm.Join(repoFullName, deploymentID)
	for _, d := range s.data.DeploymentStatus[key] {
		if d.ID == statusID {
			return d, nil, nil
		}
	}
	return nil, nil, scm.ErrNotFound
}

func (s *deploymentService) ListStatus(ctx context.Context, repoFullName string, deploymentID string, opts scm.ListOptions) ([]*scm.DeploymentStatus, *scm.Response, error) {
	key := scm.Join(repoFullName, deploymentID)
	return s.data.DeploymentStatus[key], nil, nil
}

func (s *deploymentService) CreateStatus(ctx context.Context, repoFullName string, deploymentID string, input *scm.DeploymentStatusInput) (*scm.DeploymentStatus, *scm.Response, error) {
	key := scm.Join(repoFullName, deploymentID)
	statuses := s.data.DeploymentStatus[key]

	status := &scm.DeploymentStatus{
		ID:              "status-" + strconv.Itoa(len(statuses)+1),
		State:           input.State,
		Author:          nil,
		Description:     input.Description,
		Environment:     input.Environment,
		DeploymentLink:  "",
		EnvironmentLink: input.EnvironmentLink,
		LogLink:         input.LogLink,
		RepositoryLink:  "",
		TargetLink:      input.LogLink,
		Created:         time.Time{},
		Updated:         time.Time{},
	}

	s.data.DeploymentStatus[key] = append(statuses, status)
	return status, nil, nil
}
