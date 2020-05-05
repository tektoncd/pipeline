# Builder package for tests

This package holds `Builder` functions that can be used to create struct in
tests with less noise.

One of the most important characteristic of a unit test (and any type of test
really) is **readability**. This means it should be easy to read but most
importantly it should clearly show the intent of the test. The setup (and
cleanup) of the tests should be as small as possible to avoid the noise. Those
builders exists to help with that.

There is currently two versionned builder supported:
- [`v1alpha1`](./v1alpha1)
- [`v1beta1`](./v1beta1)
