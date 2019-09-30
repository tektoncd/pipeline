#!/usr/bin/env bash

set -x

function generate_dockefiles() {
  local dockerfile_in=$1;
  local target_dir=$2; shift; shift
  for img in $@; do
    local image_base=$(basename $img)
    mkdir -p $target_dir/$image_base
    bin=$image_base envsubst < $dockerfile_in | sed 's/DOLLAR/$/g'  > $target_dir/$image_base/Dockerfile
  done
}

generate_dockefiles $@
