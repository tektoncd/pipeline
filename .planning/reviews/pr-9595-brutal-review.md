# PR #9595 Brutal Code Review: Git/HTTP Resolver Secret Hardening

**Branch**: `fix/git-resolver-secret-namespace-hardening`
**Files**: 6 changed (2 production, 4 test)
**Commits**: 3

---

## RED CIRCLE CRITICAL ISSUES (Must fix -- these will cause real problems)

### C1: `userSpecified` flag logic has a subtle semantic gap in the git-clone path

In `ResolveGitClone()` (resolver.go:198-209):

```go
secretRef := &secretCacheKey{
    name: g.Params[GitTokenParam],
    key:  g.Params[GitTokenKeyParam],
}
if secretRef.name != "" {
    if secretRef.key == "" {
        secretRef.key = DefaultTokenKeyParam
    }
    secretRef.ns = common.RequestNamespace(ctx)
} else {
    secretRef = nil
}
```

When the user specifies `gitToken` but `RequestNamespace(ctx)` returns `""`, `secretRef.ns` is set to `""`. Then in `getAPIToken`, `userSpecified = true` and the empty-namespace guard correctly blocks the fallback. **This is fine.**

However, there is a gap: if a user specifies `gitToken` with a non-empty name, but the user's `gitTokenKey` param happens to be empty, the code sets `secretRef.key = DefaultTokenKeyParam` ("token"). Then inside `getAPIToken`, the `apiSecret.key` is already non-empty, so the `apiSecret.key == ""` branch (lines 495-502) is skipped. This means the user's secret key silently defaults to "token" **even when `userSpecified = true`**. The config fallback for the key name is bypassed.

This is actually correct behavior (the default key is a reasonable default), but the code's structure makes it look like a bug -- the `DefaultTokenKeyParam` is set in the caller, then in `getAPIToken` the "config fallback" for key is dead code for the git-clone path. Not a security issue, but confusing code that will bite the next person who touches it.

**Verdict**: Not a security bug, but document the intent with a comment at line 203-204.

### C2: Debug log at line 529 still leaks the Kubernetes API error message

```go
g.Logger.Debugf("secret lookup failed: ns=%s name=%s err=%v", apiSecret.ns, apiSecret.name, err)
```

The `err` from `g.KubeClient.CoreV1().Secrets().Get()` contains the full Kubernetes API error, which can include messages like `secrets "my-secret" not found` vs `secrets "my-secret" is forbidden`. While this is at `Debug` level (not returned to the user), operators who enable debug logging should be aware this distinction exists in their logs. An attacker with log access could still enumerate secrets.

**However**: if an attacker has log access, they almost certainly already have cluster access. This is acceptable for debug-level logging. The commit message says "remove secret key name from debug log" which is accurate -- the key name was removed. The secret name remains, which is needed for operator debugging.

**Verdict**: Acceptable as-is. The error normalization in the returned error message (line 530) is the important part, and that's correct.

---

## ORANGE CIRCLE SERIOUS CONCERNS (Should fix -- complexity/performance risks)

### S1: Cache fix is correct but the test fix is a band-aid

The cache fix itself is solid: `g.Cache.Get(*apiSecret)` and `g.Cache.Add(*apiSecret, ...)` now dereference the pointer to use value equality. Since `secretCacheKey` has all-string fields, this is fully comparable and correct.

But in `pkg/remoteresolution/resolver/git/resolver_test.go:637`, the "api: token not found" test case was renamed from `"token-secret"` to `"token-secret-nonexistent"` to avoid a cache hit from a previous test case. This works, but it papers over the real problem: the `remoteresolution.Resolver` shares a single cache across all test cases because `Initialize()` has a `if r.cache == nil` guard that skips re-initialization.

This means:
1. Test ordering matters (fragile tests)
2. Any future test case that uses the same secret name+key+namespace will silently get a cached value from a previous test
3. The deprecated resolver doesn't have this problem because its `Initialize()` always creates a new cache

**Fix**: Either (a) create a fresh `Resolver` per test case, or (b) add a `FlushCache()` test helper, or (c) document the test coupling prominently.

### S2: Dead variable `ok` at line 474

```go
ok := false
```

This variable is declared but never used at this scope -- it's shadowed by `:=` at lines 519 and 533. This is pre-existing dead code, but since this PR touches the surrounding lines, it should be cleaned up.

**Fix**: Remove `ok := false` at line 474.

### S3: Error message inconsistency between git and HTTP resolvers

Git resolver returns:
- `"cannot get API token, secret not accessible: request namespace not available in context"` (empty ns)
- `"cannot get API token, secret not accessible in namespace %s"` (secret not found or key missing)

HTTP resolver returns:
- `"cannot get API token, secret not accessible: request namespace not available in context"` (empty ns)
- `"cannot get API token, secret not accessible in namespace %s"` (secret not found or key missing)

These are consistent with each other, which is good. But the error says "API token" in the HTTP resolver context where it's actually a "basic auth password". This is inherited from the original code and is misleading. Not a security issue, but confusing for users debugging failures.

---

## YELLOW CIRCLE QUESTIONABLE DECISIONS (Reconsider -- probably over-engineered)

### Q1: The `userSpecified` flag adds conceptual overhead to an already complex function

`getAPIToken` is now doing 5 things:
1. Loading config
2. Determining if the secret was user-specified
3. Filling in defaults from config
4. Caching logic
5. Fetching the secret

The `userSpecified` flag is correct and necessary for the security fix, but the function is getting long and the comment at lines 477-479 is doing heavy lifting to explain what's happening. Consider extracting the namespace resolution into a separate helper:

```go
func resolveSecretNamespace(ctx context.Context, userSpecified bool, conf ScmConfig) (string, error) {
    ns := common.RequestNamespace(ctx)
    if ns != "" {
        return ns, nil
    }
    if userSpecified {
        return "", fmt.Errorf("cannot get API token, secret not accessible: request namespace not available in context")
    }
    ns = conf.APISecretNamespace
    if ns == "" {
        ns = os.Getenv("SYSTEM_NAMESPACE")
    }
    return ns, nil
}
```

This would make the security boundary crystal clear instead of buried in a 70-line function.

### Q2: No test for the ResolveAPIGit path with user-specified token and empty namespace

The tests cover:
- `TestGetAPITokenUserSpecifiedEmptyNamespace` -- unit test for the guard itself
- `TestGetAPITokenConfigSourcedFallback` -- unit test for config path
- Integration tests with framework-injected namespaces

But there's no test for `ResolveAPIGit` calling `getAPIToken` with a user-specified token where `RequestNamespace` returns empty. The guard is tested at the `getAPIToken` level, which is fine, but a test at the `ResolveAPIGit` level would catch any future regression where the caller incorrectly sets the namespace before passing it in.

---

## GREEN CHECK What's Actually Fine

1. **The core security fix is correct.** The `userSpecified` flag cleanly separates the two trust domains (user-supplied secrets vs admin-configured secrets). User-supplied secrets are never allowed to fall back to config/system namespace. This closes the cross-namespace escalation path.

2. **Error normalization is well-done.** All three error conditions (secret not found, key not found, namespace empty) return the same generic message format to the caller. Detailed errors go to debug-level logs only. This prevents secret enumeration through error message oracle attacks.

3. **The cache fix using value dereferencing is correct.** `*apiSecret` produces a comparable value type. The LRU cache will now actually hit on matching secret references.

4. **The HTTP resolver empty namespace guard is clean and simple.** No config fallback, no SYSTEM_NAMESPACE fallback -- just `RequestNamespace(ctx)` or error. This is the right design for user-specified secrets.

5. **Test coverage is adequate for the security changes.** The new unit tests (`TestGetAPITokenUserSpecifiedEmptyNamespace`, `TestGetAPITokenConfigSourcedFallback`, `TestGetBasicAuthSecretEmptyNamespace`, `TestGetBasicAuthSecretNotFound`, `TestGetBasicAuthSecretWrongKey`) directly test the security boundaries.

6. **The PR correctly maintains backward compatibility.** Config-sourced secrets (admin-controlled) still fall back to `APISecretNamespace` then `SYSTEM_NAMESPACE`. Only user-specified secrets are blocked from fallback. Existing deployments using config-only secrets will not break.

---

## VERDICT

**Solid security hardening with correct threat model separation.** The `userSpecified` flag is the right approach, the error normalization is effective, and the cache fix is correct. The main concerns are the band-aid test fix in remoteresolution (fragile test ordering dependency) and the dead `ok` variable. The code could benefit from extracting namespace resolution into a helper for clarity, but the security properties are sound. Ship it after addressing S1 and S2.

---

## Pre-existing issues NOT introduced by this PR (for tracking)

1. **Context key collision** (`context.go:27,54`): `requestNamespaceContextKey` and `requestNameContextKey` are both `contextKey{}` -- same type, same value, will collide. `RequestName()` returns the namespace, not the name. This predates PR #9595.

2. **ClusterRole has cluster-wide secret read** (`config/resolvers/200-clusterrole.yaml`): The controller SA can read secrets in ANY namespace. Any namespace handling bug in the resolver code = full cross-namespace secret access. This is the reason the namespace guard matters so much.

3. **"API token" error message in HTTP resolver** refers to basic auth passwords. Misleading but not a security issue.
