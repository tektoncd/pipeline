#!/bin/bash

currentdir=$(dirname $(readlink -f $0))
repospecfile=${currentdir}/tekton.spec
TMPD=$(mktemp -d)
mkdir -p ${TMPD}
clean() { rm -rf ${TMPD} ;}
trap clean EXIT

set -e

curl -o ${TMPD}/output.json -s https://api.github.com/repos/tektoncd/cli/releases/latest
version=$(python3 -c "import sys, json;x=json.load(sys.stdin);print(x['tag_name'])" < ${TMPD}/output.json)
version=${version/v}

sed "s/_VERSION_/${version}/" ${repospecfile} > ${TMPD}/tekton.spec

cd ${TMPD}

curl -O -L https://github.com/tektoncd/cli/archive/v${version}.tar.gz

rpmbuild -bs tekton.spec --define "_sourcedir $PWD" --define "_srcrpmdir $PWD"

copr-cli --config=/var/secret/copr/copr build tektoncd-cli tektoncd-cli-${version}-1.src.rpm
