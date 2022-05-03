#!/usr/bin/env bash
set -eu -o pipefail

declare TEKTON_PROJECT TEKTON_VERSION RELEASE_BUCKET_OPT RELEASE_EXTRA_PATH RELEASE_FILE POST_RELEASE_FILE

# This script allows to deploy a Tekton release to the dogfooding cluster
# by creating a job in the robocat cluster. The complete flow is:
# cronjob --[json payload]--> robocat event listener --> deploy trigger template --> deploy pipelinerun
# The deploy pipelinerun users a cluster resource on the robocat cluster to deploy to dogfooding

# Prerequisites:
# - kubectl installed
# - cluster gke_tekton-nightly_europe-north1-a_robocat defined in the local kubeconfig

# Read command line options
while getopts ":p:v:b:e:f:g:c:r:" opt; do
  case ${opt} in
    p )
      TEKTON_PROJECT=$OPTARG
      ;;
    v )
      TEKTON_VERSION=$OPTARG
      ;;
    b )
      RELEASE_BUCKET_OPT=$OPTARG
      ;;
    e )
      RELEASE_EXTRA_PATH=$OPTARG
      ;;
    f )
      RELEASE_FILE=$OPTARG
      ;;
    g )
      POST_RELEASE_FILE=$OPTARG
      ;;
    c )
      CONTEXT=$OPTARG
      ;;
    r )
      CLUSTER_RESOURCE=$OPTARG
      ;;
    \? )
      echo "Invalid option: $OPTARG" 1>&2
      echo 1>&2
      echo "Usage:  deploy-release.sh -p project -v version [-b bucket] [-e extra-path] [-f file] [-g file] [-c context] [-r cluster-resource]" 1>&2
      ;;
    : )
      echo "Invalid option: $OPTARG requires an argument" 1>&2
      ;;
  esac
done
shift $((OPTIND -1))
RELEASE_FILE=${RELEASE_FILE:-""}
POST_RELEASE_FILE=${POST_RELEASE_FILE:-""}
RELEASE_EXTRA_PATH=${RELEASE_EXTRA_PATH:-""}

# Check and defaults input params
if [ -z "$TEKTON_PROJECT" ]; then
    echo "Please specify a project with -p project" 1>&2
    exit 1
fi
if [ -z "$TEKTON_VERSION" ]; then
    echo "Please specify a version with -v version" 1>&2
    exit 1
fi
RELEASE_BUCKET=${RELEASE_BUCKET_OPT:-gs://tekton-releases}
if [ -z "$RELEASE_FILE" ]; then
    if [ "$TEKTON_PROJECT" == "dashboard" ]; then
        RELEASE_FILE="tekton-dashboard-release-readonly.yaml"
    else
        RELEASE_FILE="release.yaml"
    fi
fi
if [ -z "$POST_RELEASE_FILE" ]; then
    if [ "$TEKTON_PROJECT" == "triggers" ]; then
        POST_RELEASE_FILE="interceptors.yaml"
    fi
fi
CONTEXT=${CONTEXT:-gke_tekton-nightly_europe-north1-a_robocat}
CLUSTER_RESOURCE=${CLUSTER_RESOURCE:-dogfooding-tekton-deployer}

# Deploy the release
# cat <<EOF | tee
cat <<EOF | kubectl create --cluster ${CONTEXT} -f-
apiVersion: batch/v1
kind: Job
metadata:
  generateName: tekton-deploy-${TEKTON_PROJECT}-${TEKTON_VERSION}-${CLUSTER_RESOURCE%%-*}-
  namespace: default
spec:
  template:
    spec:
      containers:
      - name: trigger
        image: curlimages/curl
        imagePullPolicy: Always
        volumeMounts:
        - mountPath: /workspace
          name: workspace
        command:
        - /bin/sh
        args:
        - -ce
        - |
          cat <<EOF > /workspace/post-body.json
          {
            "trigger-template": "tekton",
            "params": {
              "target": {
                "namespace": "tekton-pipelines",
                "cluster-resource": "$CLUSTER_RESOURCE"
              },
              "tekton": {
                "project": "$TEKTON_PROJECT",
                "version": "$TEKTON_VERSION",
                "environment": "dogfooding",
                "bucket": "$RELEASE_BUCKET",
                "file": "$RELEASE_FILE",
                "post-file": "$POST_RELEASE_FILE",
                "extra-path": "$RELEASE_EXTRA_PATH"
              },
              "plumbing": {
                "repository": "github.com/tektoncd/plumbing",
                "revision": "main"
              }
            }
          }
          EOF
          curl -d @/workspace/post-body.json http://el-tekton-cd.default.svc.cluster.local:8080
      restartPolicy: Never
      terminationGracePeriodSeconds: 30
      volumes:
      - emptyDir: {}
        name: workspace
EOF
