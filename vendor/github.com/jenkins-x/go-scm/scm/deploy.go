package scm

import (
	"context"
	"time"
)

type (
	// Deployment represents a request to deploy a version/ref/sha in some environment
	Deployment struct {
		ID                    string
		Namespace             string
		Name                  string
		Link                  string
		Sha                   string
		Ref                   string
		Task                  string
		FullName              string
		Description           string
		OriginalEnvironment   string
		Environment           string
		RepositoryLink        string
		StatusLink            string
		Author                *User
		Created               time.Time
		Updated               time.Time
		TransientEnvironment  bool
		ProductionEnvironment bool
		Payload               interface{}
	}

	// DeploymentInput the input to create a new deployment
	DeploymentInput struct {
		Ref                   string
		Task                  string
		Payload               string
		Environment           string
		Description           string
		RequiredContexts      []string
		AutoMerge             bool
		TransientEnvironment  bool
		ProductionEnvironment bool
	}

	// DeploymentStatus represents the status of a deployment
	DeploymentStatus struct {
		ID              string
		State           string
		Author          *User
		Description     string
		Environment     string
		DeploymentLink  string
		EnvironmentLink string
		LogLink         string
		RepositoryLink  string
		TargetLink      string
		Created         time.Time
		Updated         time.Time
	}

	// DeploymentStatusInput the input to creating a status of a deployment
	DeploymentStatusInput struct {
		State           string
		TargetLink      string
		LogLink         string
		Description     string
		Environment     string
		EnvironmentLink string
		AutoInactive    bool
	}

	// DeploymentService a service for working with deployments and deployment services
	DeploymentService interface {
		// Find find a deployment by id.
		Find(ctx context.Context, repoFullName string, deploymentID string) (*Deployment, *Response, error)

		// List returns a list of deployments.
		List(ctx context.Context, repoFullName string, opts *ListOptions) ([]*Deployment, *Response, error)

		// Create creates a new deployment.
		Create(ctx context.Context, repoFullName string, deployment *DeploymentInput) (*Deployment, *Response, error)

		// Delete deletes a deployment.
		Delete(ctx context.Context, repoFullName string, deploymentID string) (*Response, error)

		// FindStatus find a deployment status by id.
		FindStatus(ctx context.Context, repoFullName string, deploymentID string, statusID string) (*DeploymentStatus, *Response, error)

		// List returns a list of deployments.
		ListStatus(ctx context.Context, repoFullName string, deploymentID string, options *ListOptions) ([]*DeploymentStatus, *Response, error)

		// Create creates a new deployment.
		CreateStatus(ctx context.Context, repoFullName string, deploymentID string, deployment *DeploymentStatusInput) (*DeploymentStatus, *Response, error)
	}
)
