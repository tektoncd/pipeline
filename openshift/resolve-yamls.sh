#!/usr/bin/env bash
TMP=$(mktemp /tmp/.mm.XXXXXX)
clean() { rm -f ${TMP}; }
trap clean EXIT

# Take all yamls in $dir and generate a all in one yaml file with resolved registry image/tag
function resolve_resources() {
  local dir=$1
  local resolved_file_name=$2
  local ignores=$3
  local registry_prefix=$4
  local image_tag=${5}

  # This would get only one set of truth from the Makefile for the image lists
  #
  # % grep "^CORE_IMAGES" Makefile
  # CORE_IMAGES=./cmd/bash ./cmd/controller ./cmd/entrypoint ./cmd/gsutil ./cmd/kubeconfigwriter ./cmd/nop ./cmd/webhook ./cmd/imagedigestexporter
  # CORE_IMAGES_WITH_GIT=./cmd/creds-init ./cmd/git-init
  # to:
  #  % grep '^CORE_IMAGES' Makefile|sed -e 's/.*=//' -e 's,./cmd/,,g'|tr -d '\n'|sed -e 's/ /|/g' -e 's/^/(/' -e 's/$/)\n/'
  # (bash|controller|entrypoint|gsutil|kubeconfigwriter|nop|webhook|imagedigestexportercreds-init|git-init)
  local image_regexp=$(grep '^CORE_IMAGES' $(git rev-parse --show-toplevel)/Makefile| \
                           sed -e 's/.*=//' -e 's,./cmd/,,g'|tr '\n' ' '| \
                           sed -e 's/ /|/g' -e 's/^/(/' -e 's/|$/)\n/')

  >$resolved_file_name
  for yaml in $(find $dir -name "*.yaml" | grep -vE $ignores); do
    echo "---" >>$resolved_file_name
    if [[ -n ${image_tag} ]];then
        # This is a release format the output would look like this :
        # quay.io/openshift-pipeline/tektoncd-pipeline-bash:$image_tag
        sed -e "s%\(.* image: \)\(github.com\)\(.*\/\)\(.*\)%\1 ${registry_prefix}-\4:${image_tag}%" $yaml \
            -r -e "s,github.com/tektoncd/pipeline/cmd/${image_regexp},${registry_prefix}-\1:${image_tag},g" \
            > ${TMP}
     else
        # This is CI which get built directly to the user registry namespace i.e: $OPENSHIFT_BUILD_NAMESPACE
        # The output would look like this :
        # internal-registry:5000/usernamespace:tektoncd-pipeline-bash
        #
        # note: test image are images only used for testing not for releases
        sed -e 's%\(.* image: \)\(github.com\)\(.*\/\)\(test\/\)\(.*\)%\1\2 \3\4test-\5%' $yaml \
            -e "s%\(.* image: \)\(github.com\)\(.*\/\)\(.*\)%\1 ""$registry_prefix"'\:tektoncd-pipeline-\4%'  \
            -re "s,github.com/tektoncd/pipeline/cmd/${image_regexp},${registry_prefix}:tektoncd-pipeline-\1,g" \
            > ${TMP}
    fi

    # Adding the labels: openshift.io/cluster-monitoring on Namespace to add the cluster-monitoring
    # See: https://docs.openshift.com/container-platform/4.1/logging/efk-logging-deploying.html
    grep -q "kind: Namespace" ${TMP} && sed -i '/^metadata:/a \ \ labels:\n\ \ \ \ openshift.io/cluster-monitoring:\ \"true\"' ${TMP}

    cat ${TMP} >> $resolved_file_name

    echo >>$resolved_file_name
  done
}
