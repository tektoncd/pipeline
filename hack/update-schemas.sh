#!/bin/sh

CRD_PATH=$(dirname "${0}")/../config/300-crds
API_PATH=$(dirname "${0}")/../pkg/apis
OLDGOFLAGS="${GOFLAGS:-}"
GOFLAGS=""

TEMP_DIR_LOGS=$(mktemp -d)

for FILENAME in `find $CRD_PATH -type f`;
do
  echo "Gererating CRD schema for $FILENAME"

  # NOTE: APIs for the group tekton.dev are implemented under ./pkg/apis/pipeline,
  # while the ResolutionRequest from group resolution.tekton.dev is implemented under ./pkg/apis/resolution

  GROUP=$(grep -E '^  group:' $FILENAME)
  GROUP=${GROUP#"  group: "}
  if [ "$GROUP" = "tekton.dev" ]; then
    API_SUBDIR='pipeline'
  else
    API_SUBDIR=${GROUP%".tekton.dev"}
  fi
  #echo "GROUP: $GROUP"
  #echo "API_SUBDIR: $API_SUBDIR"

  TEMP_DIR=$(mktemp -d)
  cp -p $FILENAME $TEMP_DIR/.
  LOG_FILE=$TEMP_DIR_LOGS/log-schema-generation-$(basename $FILENAME)
    
  counter=0 limit=5
  while [ "$counter" -lt "$limit" ]; do
    # FIXME:(burigolucas): controller-gen return status 1 with message "Error: not all generators ran successfully"
    go run sigs.k8s.io/controller-tools/cmd/controller-gen@v0.17.1 \
      schemapatch:manifests=$TEMP_DIR,generateEmbeddedObjectMeta=false \
      output:dir=$CRD_PATH \
      paths=$API_PATH/$API_SUBDIR/... > $LOG_FILE 2>&1
    rc=$?
    if [ $rc -eq 0 ]; then
      break
    fi
    if grep -q 'exit status 1' $LOG_FILE; then
      echo "[WARNING] Ignoring errors/warnings from CRD schema generation, check $LOG_FILE for details"
      break
    fi
    counter="$(( $counter + 1 ))"
    if [ $counter -eq $limit ]; then
      echo "[ERROR] Failed to generate CRD schema"
      exit 1
    fi
  done

#   # NOTE: the controller-gen removes the field 'x-kubernetes-preserve-unknown-fields: true'
#   # The command below restores the field.
#   cat >$TEMP_DIR/tmp-fix <<EOF
# # One can use x-kubernetes-preserve-unknown-fields: true
# # at the root of the schema (and inside any properties, additionalProperties)
# # to get the traditional CRD behaviour that nothing is pruned, despite
# # setting spec.preserveUnknownProperties: false.
# #
# # See https://kubernetes.io/blog/2019/06/20/crd-structural-schema/
# # See issue: https://github.com/knative/serving/issues/912
# x-kubernetes-preserve-unknown-fields: true
# EOF
#   FIELD_IDENTATION="$(grep 'openAPIV3Schema:' $FILENAME | head -1)"
#   FIELD_IDENTATION="${FIELD_IDENTATION%openAPIV3Schema:}  "
#   sed -i "s/^/$FIELD_IDENTATION/" $TEMP_DIR/tmp-fix

#   if ! grep -qF "$(head -n1 $TEMP_DIR/tmp-fix)" $FILENAME; then
#     sed -i "/        openAPIV3Schema:/r $TEMP_DIR/tmp-fix" $FILENAME
#   fi

  rm -rf $TEMP_DIR
done

GOFLAGS="${OLDGOFLAGS}"
