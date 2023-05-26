# Tekton Pipelines Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release
of Tekton Pipelines. To follow these steps you'll need a checkout of
the pipelines repo, a terminal window and a text editor.

1. [Setup a context to connect to the dogfooding cluster](#setup-dogfooding-context) if you haven't already.

1. Install the [rekor CLI](https://docs.sigstore.dev/rekor/installation/) if you haven't already.

1. `cd` to root of Pipelines git checkout.

1. Select the commit you would like to build the release from (NOTE: the commit is full (40-digit) hash.)
    - Select the most recent commit on the ***main branch*** if you are cutting a major or minor release i.e. `x.0.0` or `0.x.0`
    - Select the most recent commit on the ***`release-<version number>x` branch***, e.g. [`release-v0.47.x`](https://github.com/tektoncd/pipeline/tree/release-v0.47.x) if you are patching a release i.e. `v0.47.2`.

1. Ensure the correct version of the release pipeline is installed on the cluster:

    ```bash
    kustomize build tekton | kubectl --context dogfooding replace -f -
    ```

1. Create environment variables for bash scripts in later steps.

    ```bash
    TEKTON_VERSION=# Example: v0.21.0
    TEKTON_RELEASE_GIT_SHA=# SHA of the release to be released
    ```

1. Confirm commit SHA matches what you want to release.

    ```bash
    git show $TEKTON_RELEASE_GIT_SHA
    ```

1. Create a workspace template file:

   ```bash
   cat <<EOF > workspace-template.yaml
   spec:
     accessModes:
     - ReadWriteOnce
     resources:
       requests:
         storage: 1Gi
   EOF
   ```

1. Execute the release pipeline (takes ~45 mins).

    **If you are back-porting include this flag: `--param=releaseAsLatest="false"`**

    ```bash
    tkn --context dogfooding pipeline start pipeline-release \
      --serviceaccount=release-right-meow \
      --param=gitRevision="${TEKTON_RELEASE_GIT_SHA}" \
      --param=serviceAccountPath=release.json \
      --param=versionTag="${TEKTON_VERSION}" \
      --param=releaseBucket=gs://tekton-releases/pipeline \
      --workspace name=release-secret,secret=release-secret \
      --workspace name=workarea,volumeClaimTemplateFile=workspace-template.yaml
    ```

    Accept the default values of the parameters (except for "releaseAsLatest" if backporting).

1. Watch logs of pipeline-release.

1. Once the pipeline is complete, check its results:

    ```bash
    tkn --context dogfooding pr describe <pipeline-run-name>

    (...)
    📝 Results

    NAME                    VALUE
    ∙ commit-sha            ff6d7abebde12460aecd061ab0f6fd21053ba8a7
    ∙ release-file           https://storage.googleapis.com/tekton-releases-nightly/pipeline/previous/v20210223-xyzxyz/release.yaml
    ∙ release-file-no-tag    https://storage.googleapis.com/tekton-releases-nightly/pipeline/previous/v20210223-xyzxyz/release.notag.yaml

    (...)
    ```

    The `commit-sha` should match `$TEKTON_RELEASE_GIT_SHA`.
    The two URLs can be opened in the browser or via `curl` to download the release manifests.

1. The YAMLs are now released! Anyone installing Tekton Pipelines will get the new version. Time to create a new GitHub release announcement:

    1. Choose a name for the new release! The usual pattern is "< cat breed > < famous robot >" e.g. "Ragdoll Norby". For LTS releases, add a suffix "LTS" in the name such as "< cat breed > < famous robot > LTS" e.g. "Ragdoll Norby LTS". Browse [the releases page](https://github.com/tektoncd/pipeline/releases) or run this command to check which names have already been used:

    ```bash
    curl \
      -H "Accept: application/vnd.github.v3+json" \
      https://api.github.com/repos/tektoncd/pipeline/releases\?per_page=100 \
      | jq ".[].name"
    ```

    1. Find the Rekor UUID for the release

    ```bash
    RELEASE_FILE=https://storage.googleapis.com/tekton-releases/pipeline/previous/${TEKTON_VERSION}/release.yaml
    CONTROLLER_IMAGE_SHA=$(curl $RELEASE_FILE | egrep 'gcr.io.*controller' | cut -d'@' -f2)
    REKOR_UUID=$(rekor-cli search --sha $CONTROLLER_IMAGE_SHA | grep -v Found | head -1)
    echo -e "CONTROLLER_IMAGE_SHA: ${CONTROLLER_IMAGE_SHA}\nREKOR_UUID: ${REKOR_UUID}"
    ```

    1. Create additional environment variables

    ```bash
    TEKTON_OLD_VERSION=# Example: v0.11.1
    TEKTON_RELEASE_NAME=# The release name you just chose, e.g.: "Ragdoll Norby"
    ```

    1. Execute the Draft Release Pipeline.

    ```bash
    tkn --context dogfooding pipeline start \
      --workspace name=shared,volumeClaimTemplateFile=workspace-template.yaml \
      --workspace name=credentials,secret=release-secret \
      -p package="tektoncd/pipeline" \
      -p git-revision="$TEKTON_RELEASE_GIT_SHA" \
      -p release-tag="${TEKTON_VERSION}" \
      -p previous-release-tag="${TEKTON_OLD_VERSION}" \
      -p release-name="${TEKTON_RELEASE_NAME}" \
      -p bucket="gs://tekton-releases/pipeline" \
      -p rekor-uuid="$REKOR_UUID" \
      release-draft
    ```

    1. Watch logs of create-draft-release

    1. On successful completion, a URL will be logged. Visit that URL and look through the release notes.
      1. Manually add upgrade and deprecation notices based on the generated release notes
      1. Double-check that the list of commits here matches your expectations
         for the release. You might need to remove incorrect commits or copy/paste commits
         from the release branch. Refer to previous releases to confirm the expected format.

    1. Un-check the "This is a pre-release" checkbox since you're making a legit for-reals release!

    1. Publish the GitHub release once all notes are correct and in order.

1. Create a branch for the release named `release-<version number>x`, e.g. [`release-v0.28.x`](https://github.com/tektoncd/pipeline/tree/release-v0.28.x)
   and push it to the repo https://github.com/tektoncd/pipeline.
   (This can be done on the Github UI.)
   Make sure to fetch the commit specified in `TEKTON_RELEASE_GIT_SHA` to create the released branch.
   > Background: The reason why we need to create a branch for the release named `release-<version number>x` is for future patch releases. Cherrypicked PRs for the patch release will be merged to this branch. For example, [v0.47.0](https://github.com/tektoncd/pipeline/releases/tag/v0.47.0) has been already released, but later on we found that an important [PR](https://github.com/tektoncd/pipeline/pull/6622) should have been included to that release. Therefore, we need to do a patch release i.e. v0.47.1 by [cherrypicking this PR](https://github.com/tektoncd/pipeline/pull/6622#pullrequestreview-1414804838), which will trigger [tekton-robot to create a new PR](https://github.com/tektoncd/pipeline/pull/6622#issuecomment-1539030091) to merge the changes to the [release-v0.47.x branch](https://github.com/tektoncd/pipeline/tree/release-v0.47.x).

1. If the release introduces a new minimum version of Kubernetes required,
   edit `README.md` on `main` branch and add the new requirement with in the
   "Required Kubernetes Version" section

1. Edit `releases.md` on the `main` branch, add an entry for the release.
   - In case of a patch release, replace the latest release with the new one,
     including links to docs and examples. Append the new release to the list
     of patch releases as well.
   - In case of a minor or major release, add a new entry for the
     release, including links to docs and example
   - Check if any release is EOL, if so move it to the "End of Life Releases"
     section

1. Push & make PR for updated `releases.md` and `README.md`

1. Test release that you just made against your own cluster (note `--context my-dev-cluster`):

    ```bash
    # Test latest
    kubectl --context my-dev-cluster apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
    ```

    ```bash
    # Test backport
    kubectl --context my-dev-cluster apply --filename https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.11.2/release.yaml
    ```

1. Announce the release in Slack channels #general, #announcements and #pipelines.
Optional: Add a photo of this release's "purr programmer" (someone's cat).

1. Update [the catalog repo](https://github.com/tektoncd/catalog) test infrastructure
to use the new release by updating the `RELEASE_YAML` link in [e2e-tests.sh](https://github.com/tektoncd/catalog/blob/main/test/e2e-tests.sh).

1. For major releases, the [website sync configuration](https://github.com/tektoncd/website/blob/main/sync/config/pipelines.yaml)
   to include the new release.

Congratulations, you're done!

## Setup dogfooding context

1. Configure `kubectl` to connect to
   [the dogfooding cluster](https://github.com/tektoncd/plumbing/blob/main/docs/dogfooding.md):

    ```bash
    gcloud container clusters get-credentials dogfooding --zone us-central1-a --project tekton-releases
    ```

1. Give [the context](https://kubernetes.io/docs/tasks/access-application-cluster/configure-access-multiple-clusters/)
   a short memorable name such as `dogfooding`:

   ```bash
   kubectl config rename-context gke_tekton-releases_us-central1-a_dogfooding dogfooding
   ```

1. **Important: Switch `kubectl` back to your own cluster by default.**

    ```bash
    kubectl config use-context my-dev-cluster
    ```

## Cherry-picking commits for patch releases

The easiest way to cherry-pick a commit into a release branch is to use the "cherrypicker" plugin (see https://prow.tekton.dev/plugins for documentation).
To use the plugin, comment "/cherry-pick <branch-to-cherry-pick-onto>" on the pull request containing the commits that need to be cherry-picked.
Make sure this command is on its own line, and use one comment per branch that you're cherry-picking onto.
Automation will create a pull request cherry-picking the commits into the named branch, e.g. `release-v0.47.x`.

The cherrypicker plugin isn't able to resolve merge conflicts. If there are merge conflicts, you'll have to manually cherry-pick following these steps:
1. Fetch the branch you're backporting to and check it out:
```sh
git fetch upstream <branchname>
git checkout upstream/<branchname>
```
1. (Optional) Rename the local branch to make it easier to work with:
```sh
git switch -c <new-name-for-local-branch>
```
1. Find the 40-character commit hash to cherry-pick. Note: automation creates a new commit when merging contributors' commits into main.
You'll need to use the hash of the commit created by tekton-robot.

1. [Cherry-pick](https://git-scm.com/docs/git-cherry-pick) the commit onto the branch:
```sh
git cherry-pick <commit-hash>
```
1. Resolve any merge conflicts.
1. Finish the cherry-pick:
```sh
git add <changed-files>
git cherry-pick --continue
```
1. Push your changes to your fork and open a pull request against the upstream branch.
