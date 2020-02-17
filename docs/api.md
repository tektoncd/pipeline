The document is for describing restful API.

* [Pipelines](#Pipelines)
* [PipelineRun](#PipelineRun)
* [PipelineResource](#PipelineResource)
* [Task](#Task)
* [ClusterTask](#ClusterTask)
* [TaskRun](#TaskRun)

There're three ways to invoke API:

- Run **kubectl proxy**
- Tekton dashboard proxy
- At least permission, like tekton-pipeline-admin ServiceAccount.

---
## Pipelines

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelines

method: GET

description: get pipielines list

example for request:
curl -X GET -H "Content-Type: application/json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton-pipelines/pipelines

response:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "items": [
        {
            "apiVersion": "tekton.dev/v1alpha1",
            "kind": "Pipeline",
            "metadata": {
                "annotations": {
                ...
},
                "creationTimestamp": "2019-11-20T13:40:49Z",
                "generation": 1,
                "name": "pipeline0",
                "namespace": "tekton-pipelines",
                ...
            },
            "spec": {
                ...
            }
        }
    ],
    "kind": "PipelineList",
    "metadata": {
        "continue": "",
        "resourceVersion": "51234",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton-pipelines/pipelines"
    }
}

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelines/{pipeline_name}

method: GET

description: get content of a pipeline

example for request:
curl -X GET -H "Content-Type: application/json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton-pipelines/pipelines/pipeline0

response:
{
  "apiVersion": "tekton.dev/v1alpha1",
  "kind": "Pipeline",
  "metadata": {
    "annotations": {
      ...
    },
    "creationTimestamp": "2019-11-20T13:40:49Z",
    "generation": 1,
    "name": "pipeline0",
    "namespace": "tekton-pipelines",
    "resourceVersion": "21820",
    "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton-pipelines/pipelines/pipeline0",
    "uid": "2b4e0d5a-6ee1-4ea3-a2fe-28e7ee265090"
  },
  "spec": {
    ...
  }
}

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelines/

method: POST

description: craete a pipeline within a specified namespace

example for request:
pipeline.json:
{
  "apiVersion": "tekton.dev/v1alpha1",
  "kind": "Pipeline",
  "metadata": {
    "name": "tutorial-pipeline"
  },
  "spec": {
    ...
  }
}

curl -X POST -H "Content-Type: application/json" 
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelines -d @pipeline.json

response:
return content of pipeline.json.
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelines/{pipeline_name}

method: PATCH

description: update a pipeline within a specified namespace

example for request:
If any need, update values in pipeline.json

curl -X PATCH -H "Content-Type: application/merge-patch+json" 
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelines/tutorial-pipeline -d '{"metadata":{"labels":{"name":"pr"}}}'

response:
return pipeline.  
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelines/{pipeline_name}

method: DELETE

description: delete a pipeline within a specified namespace

example for request:
curl -X DELETE -H "Content-Type: application/json" 
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelines/tutorial-pipeline

response:
no body to return
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelines/{pipeline_name}

method: PUT

description: replace the specified pipeline

If any need, update spec within an existing Pipeline, not metadata.
example for request:

pipeline.json:
{
  "apiVersion": "tekton.dev/v1alpha1",
  "kind": "Pipeline",
  "metadata": {
    "name": "tutorial-pipeline"
    "namespace"; "tekton"
    "resourceVersion": "113838",
    "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelines/tutorial-pipeline",
    "uid": "a8ea654a-0cde-47c6-8756-a4c80a9c011e"
  },
  "spec": {
    ...
  }
}
curl -X PUT -H "Content-Type: application/json" 
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelines/tutorial-pipeline -d @pipeline.json

response:
return pipeline.json
```

---
## PipelineRun

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns

method: POST

description: create pipelineRun

example for request:
pipelineeun.json:
{
  "apiVersion": "tekton.dev/v1alpha1",
  "kind": "PipelineRun",
  "metadata": {
    "name": "pipelinerun-with-parameters"
  },
  "spec": {
    "pipelineRef": {
      "name": "pipeline-with-parameters"
    },
    "params": [
      {
        "name": "context",
        "value": "/workspace/examples/microservices/leeroy-web"
      }
    ]
  }
}

curl -X POST -H "Content-Type: application/json" 
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns -d @pipelinerun.json

response:
return content of pipelinerun.json
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns

method: GET

description: get pipelineruns list in a specified namespace

example for request:
curl -X GET -H "Content-Type: application/json" 
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns

response:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "items": [
    ],
    "kind": "PipelineRunList",
    "metadata": {
        "continue": "",
        "resourceVersion": "13630183",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns"
    }
}

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns/{name}

method: GET

description: get definition of a pipelinerun in a specified namespace

example for request:
curl -X GET -H "Content-Type: application/json"
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns

response:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "items": [
        {
            "apiVersion": "tekton.dev/v1alpha1",
            "kind": "PipelineRun",
            "metadata": {
                "creationTimestamp": "2019-11-21T15:06:08Z",
                "generation": 1,
                "name": "pipelinerun-with-parameters",
                "namespace": "tekton",
                "resourceVersion": "13647720",
                "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns/pipelinerun-with-parameters",
                "uid": "fd007737-2b2e-4be5-b46b-2218c41ec499"
            },
            "spec": {
                ...
            },
            "status": {
                ...
            }
        }
    ],
    "kind": "PipelineRunList",
    "metadata": {
        "continue": "",
        "resourceVersion": "13651512",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns"
    }
}

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns?labelSelector={key}={value}

method: GET

description: get a pipelinerun in a specified namespace with labelSelector

example for request:
curl -X GET -H "Content-Type: application/json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns?labelSelector=name=mypipline

resonse:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "items": [
        {
            "apiVersion": "tekton.dev/v1alpha1",
            "kind": "PipelineRun",
            "metadata": {
                "annotations": {
                ...
},
                "creationTimestamp": "2019-11-22T07:41:32Z",
                "generation": 1,
                "labels": {
                    "name": "mypipline",
                    "tekton.dev/pipeline": "mypipeline"
                },
                "name": "mypipelinerun",
                "namespace": "tekton",
                ...
            },
            "spec": {
                ...
            },
            "status": {
                "conditions": [
                    ...
            }
        }
    ],
    "kind": "PipelineRunList",
    "metadata": {
        "continue": "",
        "resourceVersion": "84099",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns"
    }
}

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns/{name}

method: PUT

description: replace a pipelinerun

If any need, update spec within an existing Pipeline, not metadata.

example for request:
pipelinerun.json:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "kind": "PipelineRun",
    "metadata": {
        "annotations": {
            ...
        },
        "creationTimestamp": "2019-11-22T10:00:07Z",
        "generation": 1,
        "labels": {
            "name": "mypipline",
            "tekton.dev/pipeline": "mypipeline"
        },
        "name": "mypipelinerun",
        "namespace": "tekton",
        "resourceVersion": "95552",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns/mypipelinerun",
        "uid": "c6c1214c-9c17-4cf7-b9c2-8970f93791d2"
    },
    "spec": {
        ...
    },
    "status": {
        ...
    }
}

curl -X PUT -H "Content-Type: application/json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns/mypipelinerun -d @pipelinerun.json

response:
return content of pipelinerun.json
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns/{name}

method: PATCH

description: partially update a pipelinerun

If any need, update pipeline.json

example for request:

curl -X PATCH -H "Content-Type: application/merge-patch+json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns/mypipelinerun -d '{"metadata":{"labels":{"name":"mypipelinerun"}}}'

response:
return content of updated pipelinerun.
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineruns/{name}

method: DELETE

description: delete a pipelinerun in a specified namespace

example for request:
curl -X DELETE -H "Content-Type: application/json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineruns/mypipelinerun

response:
no body to return
```

```
path: api/v1/namespaces/{namespace}/pods/{name}/log?{key}={value}

method: GET

description: get log of a pod produced by a pipelinerun

example for request:
curl -X GET -H "Content-Type: application/json"
http://127.0.0.1:8001/api/v1/namespaces/tekton/pods/mypipelinerun-task1-x9gnt-pod-bd75c2/log

response:
log information
```

---
## PipelineResource

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineresources

method: POST

description: create a pipelineresource within a specified namespace

example for request:
pipelineresource.json:
{
  "apiVersion": "tekton.dev/v1alpha1",
  "kind": "PipelineResource",
  "metadata": {
    "creationTimestamp": "2019-11-21T06:40:33Z",
    "generation": 1,
    "name": "git-resource",
    "namespace": "tekton",
    ...
  },
  "spec": {
    ...
  }
}

curl -X POST -H "Content-Type: application/json" http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources -d @pipelineresource.json

response:
return content of pipelineresource.json
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineresources/

method: GET

description: get PipelineResources list in a specified namespace.

example for request:
curl -X GET -H "Content-Type: application/json"
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources

response:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "items": [
    ],
    "kind": "PipelineResourceList",
    "metadata": {
        "continue": "",
        "resourceVersion": "85488",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources"
    }
}

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineresources/{name}

method: GET

description: get information of a pipelineresource in a specified namespace

example for request:
curl -X GET -H "Content-Type: application/json"
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources/git-resource

response:
{
  "apiVersion": "tekton.dev/v1alpha1",
  "kind": "PipelineResource",
  "metadata": {
    "creationTimestamp": "2019-11-21T06:40:33Z",
    "generation": 1,
    "name": "git-resource",
    "namespace": "tekton",
    ...
  },
  "spec": {
    ...
}
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineresources/{name}

method: PUT

description: update a pipelineresource within a specified namespace

If any need, update spec within an existing Pipeline, not metadata.

example for request:
pipelineresource.json:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "kind": "PipelineResource",
    "metadata": {
        "creationTimestamp": "2019-11-22T08:16:59Z",
        "generation": 1,
        "name": "git-resource",
        "namespace": "tekton",
        "resourceVersion": "86893",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources/git-resource",
        "uid": "f7ffcf22-c1ad-437c-9f96-1dc2cd222b73"
    },
    "spec": {
        ...
    }
}

curl -X PUT -H "Content-Type: application/json"
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources/git-resource -d @pipelineresource.json

response:
return content of pipelineresource.json
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineresources/{name}

method: PATCH

description: partially update the specified pipelineresource

example for request:
curl -X PATCH -H "Content-Type: application/merge-patch+json"
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources/git-resource -d '{"metadata":{"labels":{"name":"pipelineresource"}}}'

response:
return new pipelineresource with .metadata.labels
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/pipelineresources/{name}

method: DELETE

description: delete a pipelineresource within a specified namespace

example for request:
curl -X DELETE -H "Content-Type: application/json"
http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/pipelineresources/git-resource

resonse:
no body to return
```

---
## Task

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/tasks

method: POST

description: create a task

example for request:
task.json:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "kind": "Task",
        "name": "mytask",
        "namespace": "tekton"
    },
    "spec": {
        "steps": [
            {
                "args": [
                    "echo 'foo' > /my-cache/bar"
                ],
                "command": [
                    "bash",
                    "-c"
                ],
                "image": "ubuntu",
                "name": "writesomething",
                "resources": {},
                "volumeMounts": [
                    {
                        "mountPath": "/my-cache",
                        "name": "my-cache"
                    }
                ]
            }
        ]
    }
}

curl -X POST -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks -d @task.json

response:
return content of task.json
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/tasks

method: GET

description: get Tasks list in a specified namespace

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks

response:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "items": [
        {
            "apiVersion": "tekton.dev/v1alpha1",
            "kind": "Task",
            "metadata": {
                "annotations": {
                   ...
                },
                "creationTimestamp": "2019-11-22T10:00:07Z",
                "generation": 1,
                "name": "mytask",
                "namespace": "tekton",
                "resourceVersion": "95465",
                "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask",
                "uid": "d1ea1548-1f8a-4db8-8565-dc4db465df57"
            },
            "spec": {
               ...
            }
        }
    ],
    "kind": "TaskList",
    "metadata": {
        "continue": "",
        "resourceVersion": "118148",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks"
    }
}
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/tasks/{name}

method: GET

description: get definition of a task

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask

response:
{
"apiVersion": "tekton.dev/v1alpha1",
"kind": "Task",
"metadata":{
"annotations":{
...
},
"creationTimestamp": "2019-11-22T10:00:07Z",
"generation": 1,
"name": "mytask",
"namespace": "tekton",
"resourceVersion": "95465",
"selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask",
"uid": "d1ea1548-1f8a-4db8-8565-dc4db465df57"
},
"spec":{
"steps":[
{"args":["echo 'foo' > /my-cache/bar" ], "command":["bash", "-c" ],…}
]
}
}
```


```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/tasks/{name}

method: PUT

description: replace a specified task.

If any need, update spec within an existing Pipeline, not metadata.

example for request:
task.json:
{
"apiVersion": "tekton.dev/v1alpha1",
"kind": "Task",
"metadata":{
"annotations":{
...
},
"creationTimestamp": "2019-11-22T10:00:07Z",
"generation": 1,
"name": "mytask",
"namespace": "tekton",
"resourceVersion": "95465",
"selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask",
"uid": "d1ea1548-1f8a-4db8-8565-dc4db465df57"
},
"spec":{
"steps":[
{"args":["echo 'foo' > /my-cache/bar" ], "command":["bash", "-c" ],…}
]
}
}

curl -X PUT -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask -d @task.json

response:
return task content replaced by task.json
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/tasks/{name}

method: PATCH

description: partially update the specified task

example for request:
curl -X PATCH -H 'Conten-Type: application/merge-patch+json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask -d '{"metadata":{"labels":{"name":"mytask"}}}'

response:
return content of task
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/tasks/{name}

method: DELETE

description: delete a taskrun

example for request:
curl -X DELETE -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/tasks/mytask 

response:
no body to return.
```

---
## ClusterTask

Similar to Task, but with a cluster scope.

In case of using a ClusterTask, the TaskRef kind should be added. The default kind is Task which represents a namespaced Task.

```
path: apis/tekton.dev/v1alpha1/clustertasks

method: POST

description: create a clustertask

example for request:
clustertask.json:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "kind": "ClusterTask",
        "name": "myclustertask"
    },
    "spec": {
        "steps": [
            {
                "args": [
                    "echo 'foo' > /my-cache/bar"
                ],
                "command": [
                    "bash",
                    "-c"
                ],
                "image": "ubuntu",
                "name": "writesomething",
                "resources": {},
                "volumeMounts": [
                    {
                        "mountPath": "/my-cache",
                        "name": "my-cache"
                    }
                ]
            }
        ]
    }
}

curl -X POST -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/clustertasks -d @clustertask.json

response:
return content of clustertask.json
```

```
path: apis/tekton.dev/v1alpha1/clustertasks

method: GET

description: get ClusterTasks list in all namespaces

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/clustertasks

response:
return content clustertasks list
```

```
path: apis/tekton.dev/v1alpha1/clustertasks/{name}

method: GET

description: get definition of a ClusterTask

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/clustertasks/myclustertask

response:
get content of a clustertask named myclustertask
```

```
path: apis/tekton.dev/v1alpha1/clustertasks/{name}

method: PUT

description: replace a clustertask

If any need, update spec within an existing Pipeline, not metadata.

example for request:
curl -X PUT -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/clustertasks/myclustertask -d @myclustertask.json

response:
return content of myclustertask.json
```

```
path: apis/tekton.dev/v1alpha1/clustertasks/{name}

method: PATCH

description: partially update a clustertask

example for request:
curl -X PATCH -H 'Conten-Type: application/merge-patch+json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/clustertasks/myclustertask -d '{"metadata":{"labels":{"name":"myclustertask"}}}'

response:
return content of myclustertask
```

```
path: apis/tekton.dev/v1alpha1/clustertasks/{name}

method: DELETE

description: delete a clustertask

example for request:
curl -X DELETE -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/clustertasks/myclustertask

response:
no body to return.
```

---
## TaskRun

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/taskruns/

method: POST

description: create a taskrun with the taskrun's definition as body

example for request:
taskrun.json:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "kind": "TaskRun",
    "metadata": {
        "labels": {
            "tekton.dev/task": "mytaskrun"
        },
        "name": "mytaskrun",
        "namespace": "tekton"
    },
    "spec": {
        "inputs": {
            ...
        },
        "outputs": {
            ...
        },
        "podTemplate": {},
        "serviceAccountName": "build-bot",
        "taskRef": {
            "kind": "Task",
            "name": "buildpacks-v3-java"
        }
    }
    }
    
curl -X POST -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns -d @taskrun.json

response:
return content of taskrun.json.
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/taskruns

method: GET

description: get TaskRuns list in a specified namespace

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns

response:
{
"apiVersion": "tekton.dev/v1alpha1",
"items":[
{"apiVersion": "tekton.dev/v1alpha1", "kind": "TaskRun", "metadata":{"annotations":{…}}},
{"apiVersion": "tekton.dev/v1alpha1", "kind": "TaskRun", "metadata":{"creationTimestamp": "2019-11-22T06:10:47Z",…}}
],
"kind": "TaskRunList",
"metadata":{
"continue": "",
"resourceVersion": "14293895",
"selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns"}
}
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/taskruns/{name}

method: GET

description: get definition of a taskrun

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns/mytaskrun

response:
{
    "apiVersion": "tekton.dev/v1alpha1",
    "kind": "TaskRun",
    "metadata": {
        "creationTimestamp": "2019-11-22T06:10:47Z",
        "generation": 1,
        "labels": {
            "tekton.dev/task": "java-run"
        },
        "name": "mytaskrun",
        "namespace": "tekton",
        "resourceVersion": "13849362",
        "selfLink": "/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns/mytaskrun",
        "uid": "691864f9-cb92-405b-a5f9-2fad2b085b32"
    },
    "spec": {
        ...
    },
    "status": {
        ...
    }
}
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespa}/taskruns?labelSelector={key}={value}

method: GET

description: get a TaskRun in a specified namespace with labelSelector

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns?labelSelector=tekton.dev/task=mytaskrun

response:
return taskruns list that satisfies labelSelector as tekton.dev/task=mytaskrun
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/taskruns/{name}

method: PUT

description: replace a tasktun with the taskrun's definition as body

If any need, update spec within an existing Pipeline, not metadata.

example for request:

curl -X PUT -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns/mytaskrun -d @taskrun.json

response:
return replaced taskrun
```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/taskruns/{name}

method: PATCH

description: partially update a taskrun

example for request:
curl -X PATCH -H 'Conten-Type: application/merge-patch+json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns/mytaskrun -d '{"metadata":{"labels":{"name":"mytaskrun"}}}'

response:
return updated taskrun

```

```
path: apis/tekton.dev/v1alpha1/namespaces/{namespace}/taskruns/{name}

method: DELETE

description: delete a taskrun in a specified namespace

example for request:
curl -X DELETE -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/tekton.dev/v1alpha1/namespaces/tekton/taskruns/mytaskrun

response:
no body to return.
```

```
path: apis/v1/namespaces/{namespace}/pods/{name}/log?{key}={value}

method: GET

description: get log of a pod produced by a taskrun

example for request:
curl -X GET -H 'Conten-Type: application/json' http://127.0.0.1:8001/apis/v1/namespaces/tekton/pods/mytaskrun-akba234v/log?name=mytaskrun

response:
return pod's log
```
