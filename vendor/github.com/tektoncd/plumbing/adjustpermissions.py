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

This script requires the `gcloud` command line tool and the python
`PyYaml` library.

Example usage:
  python3 adjustpermissions.py --users "foo@something.com,bar@something.com"
Or to remove access for foo@something.com and bar@something.com:
  python3 adjustpermissions.py --users "foo@something.com,bar@something.com --remove"
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
        subprocess.check_call(shlex.split(
            "gcloud projects {} {} --member user:{} --role {}".format(command, project, user, role)
        ))


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
  arg_parser.add_argument("--readonly", type=bool, default=True, help="Grant only read permissions")
  args = arg_parser.parse_args()

  gcloud_required()
  boskos_projects = parse_boskos_projects()
  update_all_projects([u.strip() for u in args.users.split(",")], list(KNOWN_PROJECTS) + boskos_projects, args.remove, args.readonly)
