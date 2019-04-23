```
apiVersion: tekton.dev/v1alpha1
kind: PipelineListener
metadata:
  name: test-build-pipeline-listener
  namespace: tekton-pipelines
spec:
  selector:
    matchLabels:
      app: test-build-pipeline-listener
  serviceName: test-build-pipeline-listener
  template:
    metadata:
      labels:
        role: test-build-pipeline-listener
    spec:
      serviceAccountName: tekton-pipelines-controller
  listener-image: github.com/tektoncd/pipeline/cmd/pipelinelistener
  event-type: com.github.checksuite
  namespace: tekton-pipelines
  port: 80
  runspec:
    pipelineRef:
      name: demo-pipeline
    trigger:
      type: manual
    serviceAccount: 'default'
    resources:
    - name: source-repo
      resourceRef:
        name: skaffold-git
    - name: web-image
      resourceRef:
        name: skaffold-image-leeroy-web
    - name: app-image
      resourceRef:
        name: skaffold-image-leeroy-app
```
