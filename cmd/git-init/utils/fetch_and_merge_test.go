package utils_test

import (
	"github.com/knative/build-pipeline/cmd/git-init/utils"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/knative/pkg/logging"

	"github.com/stretchr/testify/assert"
)

const (
	initialReadme       = "Cheesy!"
	commit1Readme       = "Yet more cheese!"
	commit2Contributing = "Even more cheese!"
	commit3License      = "It's cheesy!"
	contributing        = "CONTRIBUTING"
	readme              = "README"
	license             = "LICENSE"
)

func TestFetchAndMergeOneSHA(t *testing.T) {
	logger, _ := logging.NewLogger("", "git-init")
	// This test only uses local repos, so it's safe to use real git
	env := prepareFetchAndMergeTests(t)
	defer env.Cleanup()
	// Test merging one commit
	refs := &utils.PullRefs{
		ToMerge: map[string]string{
			"1": env.Sha1,
	},
		BaseSha: env.BaseSha,
		BaseBranch: "master",
	}
	utils.FetchAndMergeSHAs(refs, "origin", env.LocalDir, logger)
	readmeFile, err := ioutil.ReadFile(env.ReadmePath)
	assert.NoError(t, err)
	assert.Equal(t, commit1Readme, string(readmeFile))
}

func TestFetchAndMergeMultipleSHAs(t *testing.T) {
	logger, _ := logging.NewLogger("", "git-init")

	// This test only uses local repos, so it's safe to use real git
	env := prepareFetchAndMergeTests(t)
	defer env.Cleanup()

	// Test merging two commit
	refs := &utils.PullRefs{
		ToMerge: map[string]string{
			"1": env.Sha1,
			"2": env.Sha2,
		},
		BaseSha: env.BaseSha,
		BaseBranch: "master",
	}
	utils.FetchAndMergeSHAs(refs, "origin", env.LocalDir, logger)
	localContributingPath := filepath.Join(env.LocalDir, contributing)
	readmeFile, err := ioutil.ReadFile(env.ReadmePath)
	assert.NoError(t, err)
	assert.Equal(t, commit1Readme, string(readmeFile))
	contributingFile, err := ioutil.ReadFile(localContributingPath)
	assert.NoError(t, err)
	assert.Equal(t, commit2Contributing, string(contributingFile))
}

func TestFetchAndMergeSHAAgainstNonHEADSHA(t *testing.T) {

	logger, _ := logging.NewLogger("", "git-init")


	// This test only uses local repos, so it's safe to use real git
	env := prepareFetchAndMergeTests(t)
	defer env.Cleanup()

	refs := &utils.PullRefs{
		ToMerge: map[string]string{
			"3": env.Sha3,
		},
		BaseSha: env.Sha1,
		BaseBranch: "master",
	}
	// Test merging two commit
	utils.FetchAndMergeSHAs(refs, "origin", env.LocalDir, logger)

	readmeFile, err := ioutil.ReadFile(env.ReadmePath)
	assert.NoError(t, err)
	assert.Equal(t, commit1Readme, string(readmeFile))

	localContributingPath := filepath.Join(env.LocalDir, contributing)
	_, err = os.Stat(localContributingPath)
	assert.True(t, os.IsNotExist(err))

	localLicensePath := filepath.Join(env.LocalDir, license)
	licenseFile, err := ioutil.ReadFile(localLicensePath)
	assert.NoError(t, err)
	assert.Equal(t, commit3License, string(licenseFile))
}

type FetchAndMergeTestEnv struct {
	BaseSha    string
	LocalDir   string
	Sha1       string
	Sha2       string
	Sha3       string
	ReadmePath string
	Cleanup    func()
}

func prepareFetchAndMergeTests(t *testing.T) FetchAndMergeTestEnv {

	// Prepare a git repo to test - this is our "remote"
	remoteDir, err := ioutil.TempDir("", "remote")
	assert.NoError(t, err)
	_, err = utils.Run("git",remoteDir, "init")
	assert.NoError(t, err)

	readmePath := filepath.Join(remoteDir, readme)
	contributingPath := filepath.Join(remoteDir, contributing)
	licensePath := filepath.Join(remoteDir, license)
	err = ioutil.WriteFile(readmePath, []byte(initialReadme), 0600)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir,"add", readme)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "commit", "-m", "Initial Commit")
	assert.NoError(t, err)

	// Prepare another git repo, this is local repo
	localDir, err := ioutil.TempDir("", "local")
	assert.NoError(t, err)
	_, err = utils.Run("git", localDir,"init")
	assert.NoError(t, err)
	// Set up the remote
	out, err := utils.Run("git", localDir, "remote", "add", "origin", remoteDir)
	t.Log(out)
	assert.NoError(t, err)
	_, err = utils.Run("git", localDir, "fetch", "origin", "master")
	assert.NoError(t, err)
	_, err = utils.Run("git", localDir, "merge", "origin/master")
	assert.NoError(t, err)

	localReadmePath := filepath.Join(localDir, readme)
	readmeFile, err := ioutil.ReadFile(localReadmePath)
	assert.NoError(t, err)
	assert.Equal(t, initialReadme, string(readmeFile))
	baseSha, err := utils.Run("git", localDir, "rev-parse", "HEAD")
	assert.NoError(t, err)

	// Add some commits to master on the remote
	err = ioutil.WriteFile(readmePath, []byte(commit1Readme), 0600)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "add", readme)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "commit", "-m","More Cheese")
	assert.NoError(t, err)
	sha1, err := utils.Run("git", remoteDir, "rev-parse", "HEAD")
	assert.NoError(t, err)

	err = ioutil.WriteFile(contributingPath, []byte(commit2Contributing), 0600)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "add", contributing)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "commit", "-m","Even More Cheese")
	assert.NoError(t, err)
	sha2, err := utils.Run("git", remoteDir, "rev-parse", "HEAD")
	assert.NoError(t, err)

	// Put some commits on a branch
	branchName := "another"

	_, err = utils.Run("git", remoteDir, "branch", branchName, baseSha)
		assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "checkout", branchName)
	assert.NoError(t, err)

	err = ioutil.WriteFile(licensePath, []byte(commit3License), 0600)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "add", license)
	assert.NoError(t, err)
	_, err = utils.Run("git", remoteDir, "commit", "-m", "Even More Cheese")
	assert.NoError(t, err)
	sha3, err := utils.Run("git", remoteDir, "rev-parse", "HEAD")
	assert.NoError(t, err)

	return FetchAndMergeTestEnv{
		BaseSha:    baseSha,
		LocalDir:   localDir,
		Sha1:       sha1,
		Sha2:       sha2,
		Sha3:       sha3,
		ReadmePath: localReadmePath,
		Cleanup: func() {
			err := os.RemoveAll(localDir)
			assert.NoError(t, err)
			err = os.RemoveAll(remoteDir)
			assert.NoError(t, err)
		},
	}
}
