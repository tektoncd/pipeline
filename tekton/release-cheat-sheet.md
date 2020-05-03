# Tekton Pipelines Official Release Cheat Sheet

This doc is a condensed version of our [tekton/README.md](./README.md) as
well as instructions from
[our plumbing repo](https://github.com/tektoncd/plumbing/tree/master/tekton/resources/release/README.md#create-draft-release).

These steps provide a no-frills guide to performing an official release
of Tekton Pipelines. To follow these steps you'll need a checkout of
the pipelines repo, a terminal window and a text editor.

1. `cd` to root of Pipelines git checkout.

2. Point `kubectl` at dogfooding cluster.

    ```bash
    gcloud container clusters get-credentials dogfooding --zone us-central1-a --project tekton-releases
    ```

3. Edit `tekton/resources.yaml`. Add pipeline resource for new version.

    ```yaml
    apiVersion: tekton.dev/v1alpha1
    kind: PipelineResource
    metadata:
      name: # UPDATE THIS. Example: tekton-pipelines-v0-11-2
    spec:
      type: git
      params:
      - name: url
        value: https://github.com/tektoncd/pipeline
      - name: revision
        value: # UPDATE THIS. Example: 33e0847e67fc9804689e50371746c3cdad4b0a9d
    ```

4. `kubectl apply -f tekton/resources.yaml`

5. Create environment variables for bash scripts in later steps.

    ```bash
    VERSION_TAG=# UPDATE THIS. Example: v0.11.2
    GIT_RESOURCE_NAME=# UPDATE THIS. Example: tekton-pipelines-v0-11-2
    IMAGE_REGISTRY=gcr.io/tekton-releases
    ```

6. Confirm commit SHA matches what you want to release.

    ```bash
    kubectl get pipelineresource "$GIT_RESOURCE_NAME" -o=jsonpath="{'Target Revision: '}{.spec.params[?(@.name == 'revision')].value}{'\n'}"
    ```

7. Execute the release pipeline.

    **If you are backporting include this flag: `--param=releaseAsLatest="false"`**

    ```bash
    tkn pipeline start \
      --param=versionTag="${VERSION_TAG}" \
      --param=imageRegistry="${IMAGE_REGISTRY}" \
      --serviceaccount=release-right-meow \
      --resource=source-repo="${GIT_RESOURCE_NAME}" \
      --resource=bucket=pipeline-tekton-bucket \
      --resource=builtBaseImage=base-image \
      --resource=builtEntrypointImage=entrypoint-image \
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

    1. Create TaskRun YAML file to execute create-draft-release Task.

        ```yaml
        apiVersion: tekton.dev/v1beta1
        kind: TaskRun
        metadata:
          generateName: create-draft-release-run-
        spec:
          taskRef:
            name: create-draft-release
          params:
          - name: release-name
            value: # UPDATE THIS. Example: "Ragdoll Norby"
          - name: package
            value: tektoncd/pipeline
          - name: release-tag
            value: # UPDATE THIS. Example: v0.11.2
          - name: previous-release-tag
            value: # UPDATE THIS. Example: v0.11.1
          resources:
            inputs:
            - name: source
              resourceRef:
                name: # UPDATE THIS WITH PIPELINE RESOURCE YOU CREATED EARLIER. Example: tekton-pipelines-v0-11-2
            - name: release-bucket
              resourceRef:
                name: # UPDATE THIS WITH BUCKET RESOURCE NAME FROM EARLIER. Example: tekton-release-bucket-pipeline
        ```

    2. Save as `create-draft-release-run.yaml`

    3. Run Draft Release Task.

        ```bash
        kubectl create -f ./create-draft-release-run.yaml
        ```

    4. Watch logs of create-draft-release TaskRun for errors.

        ```bash
        tkn tr logs -f create-draft-release-run-# this will end with a random string of characters
        ```

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

Congratulations, you're done!
