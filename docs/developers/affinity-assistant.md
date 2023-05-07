# Affinity Assistant


[Specifying `Workspaces` in a `Pipeline`](../workspaces.md#specifying-workspace-order-in-a-pipeline-and-affinity-assistants) explains
how an affinity assistant is created when a `persistentVolumeClaim` is used as a volume source for a `workspace` in a `pipelineRun`.
Please refer to the same section for more details on the affinity assistant.

This section gives an overview of how the affinity assistant is resilient to a cluster maintenance without losing
the running `pipelineRun`. (Please refer to the issue https://github.com/tektoncd/pipeline/issues/6586 for more details.)

When a list of `tasks` share a single workspace, the affinity assistant pod gets created on a `node` along with all
`taskRun` pods. It is very common for a `pipeline` author to design a long-running tasks with a single workspace.
With these long-running tasks, a `node` on which these pods are scheduled can be cordoned while the `pipelineRun` is
still running. The tekton controller migrates the affinity assistant pod to any available `node` in a cluster along with
the rest of the `taskRun` pods sharing the same workspace.

Let's understand this with a sample `pipelineRun`:

```yaml
apiVersion: tekton.dev/v1
kind: PipelineRun
metadata:
  generateName: pipeline-run-
spec:
  workspaces:
  - name: source
    volumeClaimTemplate:
      spec:
        accessModes:
          - ReadWriteOnce
        resources:
          requests:
            storage: 10Mi
  pipelineSpec:
    workspaces:
    - name: source
    tasks:
    - name: first-task
      taskSpec:
        workspaces:
        - name: source
        steps:
        - image: alpine
          script: |
            echo $(workspaces.source.path)
            sleep 60
      workspaces:
      - name: source
    - name: last-task
      taskSpec:
        workspaces:
        - name: source
        steps:
        - image: alpine
          script: |
            echo $(workspaces.source.path)
            sleep 60
      runAfter: ["first-task"]
      workspaces:
      - name: source
```

This `pipelineRun` has two long-running tasks, `first-task` and `last-task`. Both of these tasks are sharing a single
volume with the access mode set to `ReadWriteOnce` which means the volume can be mounted to a single `node` at any
given point of time.

Create a `pipelineRun` and determine on which `node` the affinity assistant pod is scheduled:

```shell
kubectl get pods -l app.kubernetes.io/component=affinity-assistant -o wide -w
NAME                              READY   STATUS    RESTARTS   AGE   IP       NODE     NOMINATED NODE   READINESS GATES
affinity-assistant-c7b485007a-0   0/1     Pending   0          0s    <none>   <none>   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     Pending   0          0s    <none>   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     ContainerCreating   0          0s    <none>   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     ContainerCreating   0          1s    <none>   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   1/1     Running             0          5s    10.244.1.144   kind-multinode-worker1   <none>           <none>
```

Now, `cordon` that node to mark it unschedulable for any new pods:

```shell
kubectl cordon kind-multinode-worker1
node/kind-multinode-worker1 cordoned
```

The node is cordoned:

```shell
kubectl get node
NAME                           STATUS                     ROLES           AGE   VERSION
kind-multinode-control-plane   Ready                      control-plane   13d   v1.26.3
kind-multinode-worker1         Ready,SchedulingDisabled   <none>          13d   v1.26.3
kind-multinode-worker2         Ready                      <none>          13d   v1.26.3
```

Now, watch the affinity assistant pod getting transferred onto other available node `kind-multinode-worker2`:

```shell
kubectl get pods -l app.kubernetes.io/component=affinity-assistant -o wide -w
NAME                              READY   STATUS    RESTARTS   AGE   IP              NODE            NOMINATED NODE   READINESS GATES
affinity-assistant-c7b485007a-0   1/1     Running   0          49s   10.244.1.144   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   1/1     Terminating   0          70s   10.244.1.144   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   1/1     Terminating   0          70s   10.244.1.144   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     Terminating   0          70s   10.244.1.144   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     Terminating   0          70s   10.244.1.144   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     Terminating   0          70s   10.244.1.144   kind-multinode-worker1   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     Pending       0          0s    <none>          <none>          <none>           <none>
affinity-assistant-c7b485007a-0   0/1     Pending       0          1s    <none>          kind-multinode-worker2   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     ContainerCreating   0          1s    <none>          kind-multinode-worker2   <none>           <none>
affinity-assistant-c7b485007a-0   0/1     ContainerCreating   0          2s    <none>          kind-multinode-worker2   <none>           <none>
affinity-assistant-c7b485007a-0   1/1     Running             0          4s    10.244.2.144    kind-multinode-worker2   <none>           <none>
```

And, the `pipelineRun` finishes to completion:

```shell
kubectl get pr
NAME                     SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
pipeline-run-r2c7k       True        Succeeded   4m22s       2m1s

kubectl get tr
NAME                            SUCCEEDED   REASON      STARTTIME   COMPLETIONTIME
pipeline-run-r2c7k-first-task   True        Succeeded   5m16s       4m7s
pipeline-run-r2c7k-last-task    True        Succeeded   4m6s        2m56s
```
