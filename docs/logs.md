<!--
---
linkTitle: "Logs"
weight: 303
---
-->

# Execution Logs

Tekton stores execution logs for [`TaskRuns`](taskruns.md) and [`PipelineRuns`](pipelineruns.md) within
the Pod holding the containers that run the `Steps` for your `TaskRun` or `PipelineRun`.

You can get execution logs using one of the following methods:

- Get the logs [directly from the Pod](https://kubernetes.io/docs/reference/kubectl/cheatsheet/#interacting-with-running-pods).
  For example, you can use the `kubectl` as follows:

  ```bash
  # Get the Pod name from the TaskRun instance.
  kubectl get taskruns -o yaml | grep podName

  # Or, get the Pod name from the PipelineRun.
  kubectl get pipelineruns -o yaml | grep podName

  # Get the logs for all containers in the Pod.
  kubectl logs $POD_NAME --all-containers

  # Or, get the logs for a specific container in the Pod.
  kubectl logs $POD_NAME -c $CONTAINER_NAME
  kubectl logs $POD_NAME -c step-run-kubectl
  ```

- Get the logs using Tekton's [`tkn` CLI](https://github.com/tektoncd/cli).

- Get the logs using [Tekton Dashboard](https://github.com/tektoncd/dashboard).

- Configure an external service to consume and display the logs. For example, [ElasticSearch, Beats, and Kibana](https://github.com/mgreau/tekton-pipelines-elastic-tutorials).
