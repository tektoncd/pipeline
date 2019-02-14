package utils

import (
	"fmt"
	"go.uber.org/zap"
)

// FetchAndMergeSHAs merges any SHAs into the refs.BaseBranch which has a tip of refs.BaseSha,
// fetching the commits from remote for the git repo in dir. It will try to fetch individual commits (
// if the remote repo supports it - see https://github.
// com/git/git/commit/68ee628932c2196742b77d2961c5e16360734a62) otherwise it uses git remote update to pull down the
// whole repo.
func FetchAndMergeSHAs(refs *PullRefs, remote string, dir string, logger *zap.SugaredLogger)  {
	refspecs := make([]string, 0)
	for _, sha := range refs.ToMerge {
		refspecs = append(refspecs, fmt.Sprintf("%s:", sha))
	}
	refspecs = append(refspecs, refs.BaseSha)
	// First lets make sure we have the commits - remember that this is a shallow clone
	args := []string{"fetch", remote}
	for _, refspec := range refspecs {
		args = append(args, refspec)
	}
	_, err := Run("git", dir, args... )
	if err != nil {
		// This can be caused by git not being configured to allow fetching individual SHAs
		// There is not a nice way to solve this except to attempt to do a full fetch
		out, err := Run ("git", dir, "remote", "update")
		if err != nil {
			logger.Fatalf( "updating remote %s, output was %s, err was %v", remote, out, err)
		}
	}
	// Ensure we are on refs.BaseBranch
	out, err := Run("git", dir, "checkout", refs.BaseBranch)
	if err != nil {
		logger.Fatalf( "checking out %s, output was %s, err was %v", refs.BaseBranch, out, err)
	}
	// Ensure we are on the right revision
	out, err = Run ("git", dir, "reset", "--hard", refs.BaseSha)
	if err != nil {
		logger.Fatalf( "resetting %s to %s, output was %s, err was %v", refs.BaseBranch, refs.BaseSha, out, err)
	}
	out, err = Run("git", dir, "clean", "--force", "-d")
	if err != nil {
		logger.Fatalf("cleaning up the git repo, output was %s, err was %v", out)
	}
	// Now do the merges
	for _, sha := range refs.ToMerge {
		out, err := Run("git", dir,"merge", sha)
		if err != nil {

			logger.Fatalf( "merging %s into master, output was %s, err was %v", sha, out, err)
		}
	}
}


