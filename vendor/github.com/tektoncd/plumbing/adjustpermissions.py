#!/usr/bin/env python3

"""
adjustpermissions.py gives users access to the Tekton GCP projects

In order to interact with GCP resources
(https://github.com/tektoncd/plumbing/blob/main/README.md#gcp-projects)
folks sometimes need to be able to do actions like push images and view
a project in the web console.

This script will add the permissions allowed to folks on the governing board
(https://github.com/tektoncd/community/blob/main/governance.md#permissions-and-access)
to all GCP projects. It can also be used to remove permissions if needed.

The script can also be used to add "readonly" permission if the `--readonly` flag
is provided.

This script requires the `gcloud` command line tool and the python
`PyYaml` library.

Example usage:
  python3 adjustpermissions.py --users "foo@something.com,bar@something.com"
Or to remove access for foo@something.com and bar@something.com:
  python3 adjustpermissions.py --users "foo@something.com,bar@something.com --remove"

Troubleshooting:
* "The policy contains bindings with conditions..." - If you see this prompt, someone has added a conditional
  policy to the IAM policies for the project. This is not currently supported by this script. The easiest solution
  is to remove the conditional policy from the project (e.g. via the command line or via the IAM section of the
  GCP UI)
"""
import argparse
import shlex
import shutil
import subprocess
import sys
import urllib.request
import yaml
from typing import List


RW_ROLES = (
  "roles/container.admin",
  "roles/iam.serviceAccountUser",
  "roles/storage.admin",
  "roles/compute.storageAdmin",
  "roles/viewer",
)
READONLY_ROLES = ("roles/viewer",)
KNOWN_PROJECTS = (
  "tekton-releases",
  "tekton-nightly",
)
BOSKOS_CONFIG_URL = "https://raw.githubusercontent.com/tektoncd/plumbing/main/boskos/boskos-config.yaml"


def gcloud_required() -> None:
  if shutil.which("gcloud") is None:
    sys.stderr.write("gcloud binary is required; https://cloud.google.com/sdk/install")
    sys.exit(1)


def update_all_projects(users: List[str], projects: List[str], remove: bool, readonly: bool) -> None:
  command = "remove-iam-policy-binding" if remove else "add-iam-policy-binding"
  roles = READONLY_ROLES if readonly else RW_ROLES
  for user in users:
    for project in projects:
      for role in roles:
        try:
            subprocess.check_call(shlex.split(
                "gcloud projects {} {} --member user:{} --role {}".format(command, project, user, role)
            ))
        except subprocess.CalledProcessError as e:
            # when removing permissions, if the permission doesn't exist an error will be returned - but that's okay
            # since the goal is to remove it (and if the permissions list has new entries since the permissions
            # were granted, this will be hit)
            if remove:
                print("unable to run {} for project {} user {} role {}: {}".format( command, project, user, role, e),
                     file=sys.stderr
                )
            else:
                raise e


def parse_boskos_projects() -> List[str]:
  config = urllib.request.urlopen(BOSKOS_CONFIG_URL).read()
  c = yaml.safe_load(config)
  nested_config = c["data"]["config"]
  cc = yaml.safe_load(nested_config)
  return cc["resources"][0]["names"]


if __name__ == '__main__':
  arg_parser = argparse.ArgumentParser(
      description="Give a user access to all plumbing resources")
  arg_parser.add_argument("--users", type=str, required=True,
                          help="The names of the users' accounts, usually their email address, comma separated")
  arg_parser.add_argument("--remove", action="store_true",
                          help="Use this flag to remove user access instead of adding it")
  arg_parser.add_argument("--readonly", type=bool, default=False, help="Grant only read permissions")
  args = arg_parser.parse_args()

  gcloud_required()
  boskos_projects = parse_boskos_projects()
  update_all_projects([u.strip() for u in args.users.split(",")], list(KNOWN_PROJECTS) + boskos_projects, args.remove, args.readonly)
