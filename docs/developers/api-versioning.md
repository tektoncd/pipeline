# API Versioning

## Adding a new API version to a Pipelines CRD

1. Read the [Kubernetes documentation](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/)
on versioning CRDs, especially the section on
[specifying multiple versions](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#specify-multiple-versions)

1. If needed, create a new folder for the new API version under pkg/apis/pipeline.
Update codegen scripts in the "hack" folder to generate client code for the Go structs in the new folder.
Example: [#5055](https://github.com/tektoncd/pipeline/pull/5055)
    - Codegen scripts will not work correctly if there are no CRDs in the new folder, but you do not need to add the
    full Go definitions of the new CRDs.
    - Knative uses annotations on the Go structs to determine what code to generate. For example, you must annotate a
    struct with "// +k8s:openapi-gen=true" for OpenAPI schema generation.

1. Add Go struct types for the new API version. Example: [#5125](https://github.com/tektoncd/pipeline/pull/5125)
    - Consider moving any logic unrelated to the API out of pkg/apis/pipeline so it's not duplicated in
    the new folder.
    - Once this code is merged, the code in pkg/apis/pipeline will need to be kept in sync between
    the two API versions until we are ready to serve the new API version to users.

1. Implement [apis.Convertible](https://github.com/tektoncd/pipeline/blob/2f93ab2fcabcf6dcc61fe16d6ef54fcdf3424a0e/vendor/knative.dev/pkg/apis/interfaces.go#L37-L45)
for the old API version. Example: [#5202](https://github.com/tektoncd/pipeline/pull/5202)
    - Knative uses this function to generate conversion code between API versions.
    - Prefer duplicating Go structs in the new type over using type aliases. Once we move to supporting
    a new API version, we don't want to make changes to the old one.
    - Before changing the stored version of the CRD to the newer version, you must implement conversion for deprecated fields.
    This is because resources that were created with earlier stored versions will use the current stored version when they're updated.
    Deprecated fields can be serialized to a CRD's annotations. Example: [#5253](https://github.com/tektoncd/pipeline/pull/5253)

1. Add the new versions to the webhook and the CRD. Example: [#5234](https://github.com/tektoncd/pipeline/pull/5234)

1. Switch the "storage" version of the CRD to the new API version, and update the reconciler code
to use this API version. Example: [#2577](https://github.com/tektoncd/pipeline/pull/2577)

1. Update examples and documentation to use the new API version.

1. Existing objects are persisted using the storage version at the time they were created.
One way to upgrade them to the new stored version is to write a
[StorageVersionMigrator](https://kubernetes.io/docs/tasks/extend-kubernetes/custom-resources/custom-resource-definition-versioning/#upgrade-existing-objects-to-a-new-stored-version),
although we have not previously done this.
