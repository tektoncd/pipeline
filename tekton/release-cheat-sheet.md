# Tekton Pipelines Official Release Cheat Sheet

These steps provide a no-frills guide to performing an official release
of Tekton Pipelines. To follow these steps you'll need a checkout of
the pipelines repo, a terminal window and a text editor.

1. `cd` to root of Pipelines git checkout.

2. Point `kubectl` at dogfooding cluster.

    ```bash
    gcloud container clusters get-credentials dogfooding --zone us-central1-a --project tekton-releases
    ```

3. Create PipelineResource for new version. e.g.

    ```yaml
    apiVersion: tekton.dev/v1alpha1
    kind: PipelineResource
    metadata:
      name: # Example: tekton-pipelines-v0-11-2
    spec:
      type: git
      params:
      - name: url
        value: https://github.com/tektoncd/pipeline
      - name: revision
        value: # Example: 33e0847e67fc9804689e50371746c3cdad4b0a9d
    ```

4. `kubectl apply -f tekton/resources.yaml`

5. Create environment variables for bash scripts in later steps.

    ```bash
    TEKTON_VERSION=# Example: v0.11.2
    TEKTON_RELEASE_GIT_RESOURCE=# Example: tekton-pipelines-v0-11-2
    TEKTON_IMAGE_REGISTRY=gcr.io/tekton-releases
    ```

6. Confirm commit SHA matches what you want to release.

    ```bash
    kubectl get pipelineresource "$TEKTON_RELEASE_GIT_RESOURCE" -o=jsonpath="{'Target Revision: '}{.spec.params[?(@.name == 'revision')].value}{'\n'}"
    ```

7. Execute the release pipeline.

    **If you are backporting include this flag: `--param=releaseAsLatest="false"`**

    ```bash
    tkn pipeline start \
      --param=versionTag="${TEKTON_VERSION}" \
      --param=imageRegistry="${TEKTON_IMAGE_REGISTRY}" \
      --serviceaccount=release-right-meow \
      --resource=source-repo="${TEKTON_RELEASE_GIT_RESOURCE}" \
      --resource=bucket=pipeline-tekton-bucket \
      --resource=builtBaseImage=base-image \
      --resource=builtEntrypointImage=entrypoint-image \
      --resource=builtNopImage=nop-image \
      --resource=builtKubeconfigWriterImage=kubeconfigwriter-image \
      --resource=builtCredsInitImage=creds-init-image \
      --resource=builtGitInitImage=git-init-image \
      --resource=builtControllerImage=controller-image \
      --resource=builtWebhookImage=webhook-image \
      --resource=builtDigestExporterImage=digest-exporter-image \
      --resource=builtPullRequestInitImage=pull-request-init-image \
      --resource=builtGcsFetcherImage=gcs-fetcher-image \
      --resource=notification=post-release-trigger \
    pipeline-release
    ```

8. Watch logs of pipeline-release.

9. The YAMLs are now released! Anyone installing Tekton Pipelines will now get the new version. Time to create a new GitHub release announcement:

    1. Create additional environment variables

    ```bash
    TEKTON_OLD_VERSION=# Example: v0.11.1
    TEKTON_RELEASE_NAME=# Example: "Ragdoll Norby"
    TEKTON_PACKAGE=tektoncd/pipeline
    ```

    2. Execute the Draft Release task.

    ```bash
    tkn task start \
      -i source="${TEKTON_RELEASE_GIT_RESOURCE}" \
      -i release-bucket=pipeline-tekton-bucket \
      -p package="${TEKTON_PACKAGE}" \
      -p release-tag="${TEKTON_VERSION}" \
      -p previous-release-tag="${TEKTON_OLD_VERSION}" \
      -p release-name="${TEKTON_RELEASE_NAME}" \
      create-draft-release
    ```

    4. Watch logs of create-draft-release

    5. On successful completion, a URL will be logged. Visit that URL and sort the
    release notes. **Double-check that the list of commits here matches your expectations
    for the release.** You might need to remove incorrect commits or copy/paste commits
    from the release branch. Refer to previous releases to confirm the expected format.

    6. Publish the GitHub release once all notes are correct and in order.

10. Edit `README.md` on `master` branch, add entry to docs table with latest release links.

11. Push & make PR for updated `README.md`

12. **Important: Stop pointing `kubectl` at dogfooding cluster.**

    ```bash
    kubectl config use-context my-dev-cluster
    ```

13. Test release that you just made.

    ```bash
    # Test latest
    kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/latest/release.yaml
    ```

    ```bash
    # Test backport
    kubectl apply --filename https://storage.googleapis.com/tekton-releases/pipeline/previous/v0.11.2/release.yaml
    ```

14. Announce the release in Slack channels #general and #pipelines.

15. Update [the catalog repo](https://github.com/tektoncd/catalog) test infrastructure
to use the new release by updating the `RELEASE_YAML` link in [e2e-tests.sh](https://github.com/tektoncd/catalog/blob/v1beta1/test/e2e-tests.sh).

Congratulations, you're done!
