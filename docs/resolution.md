<!--
---
linkTitle: "Remote Resolution"
weight: 307
---
-->

# Tekton Pipeline Remote Resolution

Remote Resolution is a Tekton beta feature that allows users to fetch tasks and pipelines from remote sources outside the cluster. Tekton provides a few built-in resolvers that can fetch from git repositories, OCI registries etc as well as a framework for writing custom resolvers.

Remote Resolution was initially created as a separate project in [TEP-060](https://github.com/tektoncd/community/blob/main/teps/0060-remote-resource-resolution.md) and migrated into the core pipelines project in [#4710](https://github.com/tektoncd/pipeline/issues/4710).

## Getting Started Tutorial

For new users getting started with Tekton Pipeline remote resolution, check out the
[resolution-getting-started.md](./resolution-getting-started.md) tutorial.

## Configuring Built-in Resolvers

These resolvers are enabled by setting the appropriate feature flag in the `resolvers-feature-flags`
ConfigMap in the `tekton-pipelines-resolvers` namespace. See the [section in install.md](install.md#configuring-built-in-remote-task-and-pipeline-resolution) for details.

The default resolver type can be configured by the `default-resolver-type` field in the `config-defaults` ConfigMap (`alpha` feature). See [additional-configs.md](./additional-configs.md) for details.

## Developer Howto: Writing a Resolver From Scratch

For a developer getting started with writing a new Resolver, see
[how-to-write-a-resolver.md](./how-to-write-a-resolver.md) and the
accompanying [resolver-template](./resolver-template).

## Resolver Reference: The interfaces and methods to implement

For a table of the interfaces and methods a resolver must implement
along with those that are optional, see [resolver-reference.md](./resolver-reference.md).

## Resolver Cache Configuration

The resolver cache is used to avoid resolution failures due to rate limiting from external services when resolving resources with bundle, git, and cluster resolvers. By default, the cache uses:
- 5 minutes ("5m") as the time-to-live (TTL) for cache entries
- 1000 entries as the maximum cache size

You can override these defaults by adding cache configuration to individual resolver ConfigMaps. The bundle, git, and cluster resolvers can have their own cache settings by adding the following keys to their respective ConfigMaps:

- `cache-max-size`: Set the maximum number of cache entries (e.g., "500", "2000")
- `cache-default-ttl`: Set the default TTL for cache entries (e.g., "10m", "30s", "1h")
- `default-cache-mode`: Set the default cache mode when no cache parameter is specified in ResolutionRequests (e.g., "always", "never", "auto")

### Cache Modes

The cache behavior is controlled by the `cache` mode, which can be set either:
1. **Per-request**: In the ResolutionRequest `cache` parameter (highest priority)
2. **Per-resolver**: In the resolver's ConfigMap `default-cache-mode` setting
3. **Fallback**: If neither is specified, defaults to `auto` for git/bundle resolvers, `never` for cluster resolver

| Cache Mode | Git Resolver | Bundle Resolver | Cluster Resolver |
|------------|--------------|-----------------|------------------|
| `always` | Always cache | Always cache | Always cache |
| `never` | Never cache | Never cache | Never cache |
| `auto` | Cache commit SHAs only | Cache digest refs only | Never cache (no immutable refs) |

### Cache Precedence

The cache mode is determined by the following precedence order:
1. **Task/Pipeline parameter**: If the `cache` parameter is specified in a ResolutionRequest, it takes highest priority
2. **ConfigMap default**: If `default-cache-mode` is set in the resolver's ConfigMap, it applies when no task parameter is provided
3. **System default**: If neither is specified, git and bundle resolvers default to `auto`, cluster resolver defaults to `never`

**Example precedence:**
```yaml
# TaskRun with explicit cache parameter (overrides ConfigMap)
apiVersion: tekton.dev/v1
kind: TaskRun
spec:
  taskRef:
    resolver: git
    params:
    - name: cache
      value: "never"  # <-- This overrides ConfigMap default-cache-mode
    - name: url
      value: https://github.com/example/repo.git
```

**Examples:**

To configure cache settings for the bundle resolver, edit the `bundleresolver-config` ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: bundleresolver-config
  namespace: tekton-pipelines-resolvers
data:
  default-service-account: "default"
  default-kind: "task"
  # Cache configuration
  cache-max-size: "2000"
  cache-default-ttl: "10m"
  default-cache-mode: "always"
```

To configure cache settings for the git resolver, edit the `git-resolver-config` ConfigMap:
```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: git-resolver-config
  namespace: tekton-pipelines-resolvers
data:
  default-revision: "main"
  fetch-timeout: "1m"
  # Cache configuration
  cache-max-size: "1500"
  cache-default-ttl: "15m"
  default-cache-mode: "auto"
```

If these values are missing or invalid, the defaults will be used. Each of these resolvers can have different cache settings, allowing you to optimize cache behavior based on the specific needs of each resolver type.

---

Except as otherwise noted, the content of this page is licensed under the
[Creative Commons Attribution 4.0 License](https://creativecommons.org/licenses/by/4.0/),
and code samples are licensed under the
[Apache 2.0 License](https://www.apache.org/licenses/LICENSE-2.0).
