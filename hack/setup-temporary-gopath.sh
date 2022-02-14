#!/usr/bin/env bash

set -o errexit
set -o nounset

# Conditionally create a temporary GOPATH for codegen
# and openapigen to execute in. This is only done if
# the current repo directory is not within GOPATH.
function shim_gopath() {
  local REPO_DIR=$(git rev-parse --show-toplevel)
  local TEMP_GOPATH="${REPO_DIR}/.gopath"
  local TEMP_TEKTONCD="${TEMP_GOPATH}/src/github.com/tektoncd"
  local TEMP_PIPELINE="${TEMP_TEKTONCD}/pipeline"
  local NEEDS_MOVE=1

  # Checks if GOPATH exists without triggering nounset panic.
  EXISTING_GOPATH=${GOPATH:-}

  # Check if repo is in GOPATH already and return early if so.
  # Unfortunately this doesn't respect a repo that's symlinked into
  # GOPATH and will create a temporary anyway. I couldn't figure out
  # a way to get the absolute path to the symlinked repo root.
  if [ ! -z $EXISTING_GOPATH ] ; then
    case $REPO_DIR/ in
      $EXISTING_GOPATH/*) NEEDS_MOVE=0;;
      *) NEEDS_MOVE=1;;
    esac
  fi

  if [ $NEEDS_MOVE -eq 0 ]; then
    return
  fi

  echo "You appear to be running from outside of GOPATH."
  echo "This script will create a temporary GOPATH at $TEMP_GOPATH for code generation."

  # Ensure that the temporary pipelines symlink doesn't exist
  # before proceeding.
  delete_pipeline_repo_symlink

  mkdir -p "$TEMP_TEKTONCD"
  # This will create a symlink from
  # (repo-root)/.gopath/src/github.com/tektoncd/pipeline
  # to the user's pipeline checkout.
  ln -s "$REPO_DIR" "$TEMP_TEKTONCD"
  echo "Moving to $TEMP_PIPELINE"
  cd "$TEMP_PIPELINE"
  export GOPATH="$TEMP_GOPATH"
}

# Helper that wraps deleting the temp pipelines repo symlink
# and prints a message about deleting the temp GOPATH.
#
# Why doesn't this func just delete the temp GOPATH outright?
# Because it might be reused across multiple hack scripts and many
# packages seem to be installed into GOPATH with read-only
# permissions, requiring sudo to delete the directory. Rather
# than surprise the dev with a password entry at the end of the
# script's execution we just print a message to let them know.
function shim_gopath_clean() {
  local REPO_DIR=$(git rev-parse --show-toplevel)
  local TEMP_GOPATH="${REPO_DIR}/.gopath"
  if [ -d "$TEMP_GOPATH" ] ; then
    # Put the user back at the root of the pipelines repo
    # after all the symlink shenanigans.
    echo "Moving to $REPO_DIR"
    cd "$REPO_DIR"
    delete_pipeline_repo_symlink
    echo "When you are finished with codegen you can safely run the following:"
    echo "sudo rm -rf \".gopath\""
   fi
}

# Delete the temp symlink to pipelines repo from the temp GOPATH dir.
function delete_pipeline_repo_symlink() {
  local REPO_DIR=$(git rev-parse --show-toplevel)
  local TEMP_GOPATH="${REPO_DIR}/.gopath"
  if [ -d "$TEMP_GOPATH" ] ; then
    local REPO_SYMLINK="${TEMP_GOPATH}/src/github.com/tektoncd/pipeline"
    if [ -L $REPO_SYMLINK ] ; then
      echo "Deleting symlink to pipelines repo $REPO_SYMLINK"
      rm -f "${REPO_SYMLINK}"
    fi
  fi
}
