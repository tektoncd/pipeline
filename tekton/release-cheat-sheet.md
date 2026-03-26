# Tekton Pipelines Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release
of Tekton Pipelines.

## Automated Releases (Pipelines-as-Code)

Releases are automated using [Pipelines-as-Code](https://pipelinesascode.com).
The release PipelineRuns are defined in the `.tekton/` directory and executed
on the shared Tekton infrastructure cluster.

### Initial Release (major/minor)

1. Select the commit you would like to build the release from on the `main` branch.

1. Create a release branch named `release-v<major>.<minor>.x`, e.g. `release-v1.10.x`,
   from that commit and push it:

   ```bash
   git checkout -b release-v1.10.x <commit-sha>
   git push upstream release-v1.10.x
   ```

   Pipelines-as-Code automatically detects the branch creation and triggers the
   release pipeline (`.tekton/release.yaml`). The version is derived from the
   branch name (`release-v1.10.x` → `v1.10.0`).

1. Monitor the PipelineRun on the [Tekton Dashboard](https://tekton.infra.tekton.dev/#/namespaces/releases-pipeline/pipelineruns)
   or via `tkn pac logs -n releases-pipeline -L`.

1. On successful completion, a draft GitHub release is created automatically.
   Visit https://github.com/tektoncd/pipeline/releases and review the draft:
   - Manually add upgrade and deprecation notices
   - Double-check the list of commits matches expectations
   - Un-check "This is a pre-release"
   - Publish the release

### Patch Release (bugfix)

Patch releases can be triggered manually or automatically.

#### Manual trigger (workflow_dispatch)

1. Go to [Actions → Patch Release](https://github.com/tektoncd/pipeline/actions/workflows/patch-release.yaml)
1. Click **Run workflow**
1. Fill in the release branch (e.g. `release-v1.10.x`) and version (e.g. `v1.10.1`)
1. Set "Publish as latest release" appropriately
1. Click **Run workflow**

The workflow triggers the release pipeline via PAC incoming webhook.

#### Automatic trigger (weekly cron)

A cron job runs every Thursday at 10:00 UTC. It scans all active release
branches (≥ v1.0) for commits since the last tag. If new commits are found,
it automatically triggers a patch release via the PAC incoming webhook.

#### After the patch release completes

1. A draft GitHub release is created automatically. Review and publish it
   as described above.

## Post-Release Steps

1. If the release introduces a new minimum version of Kubernetes required,
   edit `README.md` on `main` branch and add the new requirement in the
   "Required Kubernetes Version" section.

1. Edit `releases.md` on the `main` branch, add an entry for the release.
   - In case of a patch release, replace the latest release with the new one,
     including links to docs and examples. Append the new release to the list
     of patch releases as well.
   - In case of a minor or major release, add a new entry for the release,
     including links to docs and examples.
   - Check if any release is EOL, if so move it to the "End of Life Releases"
     section.

1. Push & make PR for updated `releases.md` and `README.md`.

1. Test release against your own cluster:

    ```bash
    # Test latest
    kubectl apply --filename https://infra.tekton.dev/tekton-releases/pipeline/latest/release.yaml

    # Test backport
    kubectl apply --filename https://infra.tekton.dev/tekton-releases/pipeline/previous/v1.10.1/release.yaml
    ```

1. Announce the release in Slack channels #general, #announcements and #pipelines.

1. Update [the plumbing repo](https://github.com/tektoncd/plumbing/blob/main/tekton/cd/pipeline/overlays/oci-ci-cd/kustomization.yaml)
   to deploy the latest version to the dogfooding cluster.

1. For major releases, update the [website sync configuration](https://github.com/tektoncd/website/blob/main/sync/config/pipelines.yaml)
   to include the new release.

## Cherry-picking commits for patch releases

The easiest way to cherry-pick a commit into a release branch is to use the
"cherrypicker" plugin. Comment `/cherry-pick <branch>` on the pull request
containing the commits that need to be cherry-picked. Use one comment per
branch. Automation will create a pull request cherry-picking the commits.

If there are merge conflicts, manually cherry-pick:

```sh
git fetch upstream <branchname>
git checkout upstream/<branchname>
git cherry-pick <commit-hash>
# Resolve conflicts, then:
git add <changed-files>
git cherry-pick --continue
git push <your-fork> HEAD:<new-branch>
# Open PR against upstream/<branchname>
```

## Release Names

Choose a name following the pattern "< cat breed > < famous robot >".
For LTS releases, add "LTS" suffix. Generate a name:

```bash
go run tekton/release_names.go
```

## Infrastructure

- **PAC controller**: https://pac.infra.tekton.dev
- **Tekton Dashboard**: https://tekton.infra.tekton.dev
- **Release namespace**: `releases-pipeline`
- **Release bucket**: `tekton-releases` (Oracle Cloud Storage)
- **Image registry**: `ghcr.io/tektoncd/pipeline`
