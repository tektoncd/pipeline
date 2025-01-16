#!/bin/sh

CRD_PATH=./config/300-crds

for FILENAME in `find $CRD_PATH -type f`;
do
    echo "Processing file $FILENAME"

    # NOTE: APIs for the group tekton.dev are implemented under ./pkg/apis/pipeline,
    # while the ResolutionRequest from group resolution.tekton.dev is implemented under ./pkg/apis/resolution

    GROUP=$(grep -E '^  group:' $FILENAME)
    GROUP=${GROUP#"  group: "}
    if [ "$GROUP" = "tekton.dev" ]; then
      API_SUBDIR='pipeline'
    else
      API_SUBDIR=${GROUP%".tekton.dev"}
    fi
    echo "GROUP: $GROUP"
    echo "API_SUBDIR: $API_SUBDIR"

    TEMP_DIR=$(mktemp -d)
    cp -p $FILENAME $TEMP_DIR/.
    go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1 \
      schemapatch:manifests=$TEMP_DIR,generateEmbeddedObjectMeta=false \
      output:dir=$CRD_PATH \
      paths=./pkg/apis/$API_SUBDIR/...

    # NOTE: the controller-gen removes the field 'x-kubernetes-preserve-unknown-fields: true'
    # The command below restores the field.
    cat >$TEMP_DIR/tmp-fix <<EOF
# One can use x-kubernetes-preserve-unknown-fields: true
# at the root of the schema (and inside any properties, additionalProperties)
# to get the traditional CRD behaviour that nothing is pruned, despite
# setting spec.preserveUnknownProperties: false.
#
# See https://kubernetes.io/blog/2019/06/20/crd-structural-schema/
# See issue: https://github.com/knative/serving/issues/912
x-kubernetes-preserve-unknown-fields: true
EOF
    FIELD_IDENTATION="$(grep 'openAPIV3Schema:' $FILENAME | head -1)"
    FIELD_IDENTATION="${FIELD_IDENTATION%openAPIV3Schema:}  "
    sed -i "s/^/$FIELD_IDENTATION/" $TEMP_DIR/tmp-fix

    if ! grep -qF "$(head -n1 $TEMP_DIR/tmp-fix)" $FILENAME; then
        sed -i "/        openAPIV3Schema:/r $TEMP_DIR/tmp-fix" $FILENAME
    fi

    rm -rf $TEMP_DIR
done
