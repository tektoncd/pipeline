#!/usr/bin/env bash
set -eu -o pipefail

RELEASE_BRANCH=${1:-$(git rev-parse --abbrev-ref HEAD)}
shift

echo "> Make sure our remotes are up-to-date"
git fetch -p --all

TEKTON_RELEASE_GIT_SHA=$(git rev-parse "${RELEASE_BRANCH}")
TEKTON_OLD_VERSION=$(git describe --tags --abbrev=0 "${TEKTON_RELEASE_GIT_SHA}")
TEKTON_OLD_VERSION_COMMIT_SHA=$(git rev-list -n 1 "${TEKTON_OLD_VERSION}")
TEKTON_RELEASE_NAME=$(gh release view "${TEKTON_OLD_VERSION}" --json name | jq .name | sed -e 's/.*\\"\(.*\)\\"\"/\1/')

if [[ "${TEKTON_RELEASE_GIT_SHA}" == "${TEKTON_OLD_VERSION_COMMIT_SHA}" ]]; then
    echo "> No new commit in ${RELEASE_BRANCH} (${TEKTON_RELEASE_GIT_SHA}==${TEKTON_OLD_VERSION_COMMIT_SHA})"
    exit 0
fi

TEKTON_VERSION=$(echo ${TEKTON_OLD_VERSION} | awk -F. -v OFS=. '{$NF += 1 ; print}')

echo "> Old version : ${TEKTON_OLD_VERSION}"
echo "> Old version commit : ${TEKTON_OLD_VERSION_COMMIT_SHA}"
echo "> New version : ${TEKTON_VERSION}"
echo "> New version commit: ${TEKTON_RELEASE_GIT_SHA}"
echo "> Tekton Release Name: ${TEKTON_RELEASE_NAME}"

# Might be overkill
git --no-pager diff "${TEKTON_OLD_VERSION_COMMIT_SHA}" "${TEKTON_RELEASE_GIT_SHA}"

cat <<EOF > workspace-template.yaml
spec:
  accessModes:
  - ReadWriteOnce
  resources:
    requests:
      storage: 1Gi
EOF

echo "> Starting the release pipeline"
tkn pipeline start pipeline-release \
  --serviceaccount=release-right-meow \
  --param=gitRevision="${TEKTON_RELEASE_GIT_SHA}" \
  --param=serviceAccountPath=release.json \
  --param=versionTag="${TEKTON_VERSION}" \
  --param=releaseBucket=gs://tekton-releases/pipeline \
  --param=releaseAsLatest="false" \
  --workspace name=release-secret,secret=release-secret \
  --workspace name=workarea,volumeClaimTemplateFile=workspace-template.yaml --use-param-defaults --pipeline-timeout 3h --showlog

RELEASE_FILE=https://storage.googleapis.com/tekton-releases/pipeline/previous/${TEKTON_VERSION}/release.yaml
CONTROLLER_IMAGE_SHA=$(curl $RELEASE_FILE | egrep 'gcr.io.*controller' | cut -d'@' -f2)
REKOR_UUID=$(rekor-cli search --sha $CONTROLLER_IMAGE_SHA | grep -v Found | head -1)
echo -e "CONTROLLER_IMAGE_SHA: ${CONTROLLER_IMAGE_SHA}\nREKOR_UUID: ${REKOR_UUID}"

echo "> Starting the release-draft pipeline"
tkn pipeline start release-draft \
  --workspace name=shared,volumeClaimTemplateFile=workspace-template.yaml \
  --workspace name=credentials,secret=release-secret \
  -p package="tektoncd/pipeline" \
  -p git-revision="${TEKTON_RELEASE_GIT_SHA}" \
  -p release-tag="${TEKTON_VERSION}" \
  -p previous-release-tag="${TEKTON_OLD_VERSION}" \
  -p release-name="${TEKTON_RELEASE_NAME}" \
  -p bucket="gs://tekton-releases/pipeline" \
  -p rekor-uuid="$REKOR_UUID" \
  --showlog
