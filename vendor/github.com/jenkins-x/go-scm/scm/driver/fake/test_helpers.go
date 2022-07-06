package fake

import (
	"context"
	"testing"

	"github.com/jenkins-x/go-scm/scm"
	"github.com/stretchr/testify/require"
)

// AssertRepoExists asserts that the repository exists
func AssertRepoExists(ctx context.Context, t *testing.T, client *scm.Client, repo string) *scm.Repository {
	require.NotEmpty(t, repo, "no repository name")
	require.NotNil(t, client, "no scm client")
	require.NotNil(t, client.Repositories, "scm client does not support Repositories")

	repository, _, err := client.Repositories.Find(ctx, repo)

	if err != nil && scm.IsScmNotFound(err) {
		err = nil
	}
	require.NoError(t, err, "failed to find repo %s", repo)
	require.NotNil(t, repository, "no repository returned for %s", repo)
	return repository
}

// AssertNoRepoExists asserts that the repository does not exist
func AssertNoRepoExists(ctx context.Context, t *testing.T, client *scm.Client, repo string) {
	require.NotEmpty(t, repo, "no repository name")
	require.NotNil(t, client, "no scm client")
	require.NotNil(t, client.Repositories, "scm client does not support Repositories")

	_, _, err := client.Repositories.Find(ctx, repo)
	require.Error(t, err, "expected not found error when looking up repo %s", repo)
	require.True(t, scm.IsScmNotFound(err), "should have returned an is not found error for repo %s", repo)
}
