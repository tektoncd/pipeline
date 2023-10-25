# Feature Versioning

The stability levels of features (feature versioning) are independent of CRD API versions (API versioning).

## Adding feature gates for API-driven features
API-driven features are features that are enabled via a specific field in pipeline API. They comply to the [feature gates](../../api_compatibility_policy.md#feature-gates) and the [feature graduation process](../../api_compatibility_policy.md#feature-graduation-process) specified in the [API compatibility policy](../../api_compatibility_policy.md). For example, [remote tasks](https://github.com/tektoncd/pipeline/blob/454bfd340d102f16f4f2838cf4487198537e3cfa/docs/taskruns.md#remote-tasks) is an API-driven feature.
### `enable-api-fields`

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
// EnableAlphaAPIFields enables alpha features in an existing context (for use in testing)
func EnableAlphaAPIFields(ctx context.Context) context.Context {
	return setEnableAPIFields(ctx, config.AlphaAPIFields)
}

func setEnableAPIFields(ctx context.Context, want string) context.Context {
	featureFlags, _ := config.NewFeatureFlagsFromMap(map[string]string{
		"enable-api-fields": want,
	})
	cfg := &config.Config{
		Defaults: &config.Defaults{
			DefaultTimeoutMinutes: 60,
		},
		FeatureFlags: featureFlags,
	}
	return config.ToContext(ctx, cfg)
}
```

### Example YAMLs

Writing new YAML examples that require a feature gate to be set is easy. New
YAML example files typically go in a directory called something like
`examples/v1/taskruns` in the root of the repo. To create a YAML that
should only be exercised when the `enable-api-fields` flag is `alpha` just put
it in an `alpha` subdirectory so the structure looks like:

```
examples/v1/taskruns/alpha/your-example.yaml
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
