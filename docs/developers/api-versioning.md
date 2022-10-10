# API Versioning

## Adding feature gated API fields

We've introduced a feature-flag called `enable-api-fields` to the
[config-feature-flags.yaml file](../../config/config-feature-flags.yaml)
deployed as part of our releases.

This field can be configured either to be `alpha`, `beta`, or `stable`. This field is
documented as part of our
[install docs](../install.md#customizing-the-pipelines-controller-behavior).

For developers adding new features to Pipelines' CRDs we've got a couple of
helpful tools to make gating those features simpler and to provide a consistent
testing experience.

### Guarding Features with Feature Gates

Writing new features is made trickier when you need to support both the existing
stable behaviour as well as your new alpha behaviour.

In reconciler code you can guard your new features with an `if` statement such
as the following:

```go
alphaAPIEnabled := config.FromContextOrDefaults(ctx).FeatureFlags.EnableAPIFields == "alpha"
if alphaAPIEnabled {
  // new feature code goes here
} else {
  // existing stable code goes here
}
```

Notice that you'll need a context object to be passed into your function for
this to work. When writing new features keep in mind that you might need to
include this in your new function signatures.

### Guarding Validations with Feature Gates

Just because your application code might be correctly observing the feature gate
flag doesn't mean you're done yet! When a user submits a Tekton resource it's
validated by Pipelines' webhook. Here too you'll need to ensure your new
features aren't accidentally accepted when the feature gate suggests they
shouldn't be. We've got a helper function,
[`ValidateEnabledAPIFields`](../../pkg/apis/version/version_validation.go),
to make validating the current feature gate easier. Use it like this:

```go
requiredVersion := config.AlphaAPIFields
// errs is an instance of *apis.FieldError, a common type in our validation code
errs = errs.Also(ValidateEnabledAPIFields(ctx, "your feature name", requiredVersion))
```

If the user's cluster isn't configured with the required feature gate it'll
return an error like this:

```
<your feature> requires "enable-api-fields" feature gate to be "alpha" but it is "stable"
```

### Unit Testing with Feature Gates

Any new code you write that uses the `ctx` context variable is trivially unit
tested with different feature gate settings. You should make sure to unit test
your code both with and without a feature gate enabled to make sure it's
properly guarded. See the following for an example of a unit test that sets the
feature gate to test behaviour:

```go
featureFlags, err := config.NewFeatureFlagsFromMap(map[string]string{
        "enable-api-fields": "alpha",
})
if err != nil {
	t.Fatalf("unexpected error initializing feature flags: %v", err)
}
cfg := &config.Config{
        FeatureFlags: featureFlags,
}
ctx := config.ToContext(context.Background(), cfg)
if err := ts.TestThing(ctx); err != nil {
	t.Errorf("unexpected error with alpha feature gate enabled: %v", err)
}
```

### Example YAMLs

Writing new YAML examples that require a feature gate to be set is easy. New
YAML example files typically go in a directory called something like
`examples/v1beta1/taskruns` in the root of the repo. To create a YAML that
should only be exercised when the `enable-api-fields` flag is `alpha` just put
it in an `alpha` subdirectory so the structure looks like:

```
examples/v1beta1/taskruns/alpha/your-example.yaml
```

This should work for both taskruns and pipelineruns.

**Note**: To execute alpha examples with the integration test runner you must
manually set the `enable-api-fields` feature flag to `alpha` in your testing
cluster before kicking off the tests.

When you set this flag to `stable` in your cluster it will prevent `alpha`
examples from being created by the test runner. When you set the flag to `alpha`
all examples are run, since we want to exercise backwards-compatibility of the
examples under alpha conditions.

### Integration Tests

For integration tests we provide the
[`requireAnyGate` function](../../test/gate.go) which should be passed to the
`setup` function used by tests:

```go
c, namespace := setup(ctx, t, requireAnyGate(map[string]string{"enable-api-fields": "alpha"}))
```

This will Skip your integration test if the feature gate is not set to `alpha`
with a clear message explaining why it was skipped.

**Note**: As with running example YAMLs you have to manually set the
`enable-api-fields` flag to `alpha` in your test cluster to see your alpha
integration tests run. When the flag in your cluster is `alpha` _all_
integration tests are executed, both `stable` and `alpha`. Setting the feature
flag to `stable` will exclude `alpha` tests.

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