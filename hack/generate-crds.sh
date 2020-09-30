#!/usr/bin/env bash

set -e
set -o errexit
set -o nounset
set -o pipefail

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../" && pwd)"
PREFIX=${GOBIN:-${GOPATH:-$HOME/go}/bin}
API_PATH="$1"
CRD_PATH="$2"
TMP_DIR=$(mktemp -d)

echo "Update Controller-gen and kustomize"
# go get -u updates the go.mod & go.sum file so we install it from another directory
cd "$(mktemp -d)"
cp $PROJECT_ROOT/go.* .
go get -u sigs.k8s.io/controller-tools/cmd/controller-gen@v0.4.0
go get -u sigs.k8s.io/kustomize/kustomize/v3
cd "$PROJECT_ROOT"

echo "Creating CRDs at $TMP_DIR"
"${PREFIX}/controller-gen" \
    crd \
    paths="$API_PATH" \
    output:dir="$TMP_DIR"

cd "$TMP_DIR"
for filename in *.yaml; do
    cd "$TMP_DIR"
    COMMON_SECTION="
resources:
- $filename

commonLabels:
    app.kubernetes.io/instance: default
    app.kubernetes.io/part-of: tekton-pipelines
    pipeline.tekton.dev/release: devel
    version: devel

"

    mkdir -p "$TMP_DIR/generate-$filename"
    mkdir -p "$TMP_DIR/generate-$filename/patches"

    if [ -f "$PROJECT_ROOT/hack/crd-patches/$filename" ]; then
        echo "Patching $filename"
        cp "$PROJECT_ROOT/hack/crd-patches/$filename" "$TMP_DIR/generate-$filename/patches"

        cat << EOF > "$TMP_DIR/generate-$filename/kustomization.yaml"
$COMMON_SECTION

patches:
- patches/$filename
EOF
    else
        echo "No patch for $filename"
        echo "$COMMON_SECTION" > "$TMP_DIR/generate-$filename/kustomization.yaml"
    fi
    cp "$filename" "$TMP_DIR/generate-$filename"

    cd "$TMP_DIR/generate-$filename"
    kustomize build -o "$CRD_PATH/300-$filename"    
done;

rm -rf "$TMP_DIR"
