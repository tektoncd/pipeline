# Testing Best Practices

This document outlines best practices for writing unit tests in the Tekton Pipeline project.

## Error Handling in Tests: `t.Fatalf` vs `t.Errorf`

In Go tests, both `t.Fatalf` and `t.Errorf` are used to report test failures, but they behave differently:

* `t.Errorf` → reports an error and continues execution
* `t.Fatalf` → reports an error and stops the test immediately

Choosing the right one improves test clarity, reliability, and debugging experience.

## When to Use `t.Fatalf()`

Stop the test immediately when continuing is **impossible or unsafe**.

### Rule: Stop Immediately If...

1. **Test setup fails** - Prerequisites aren't met
2. **Critical preconditions fail** - Test assumptions are violated  
3. **Continuing would cause panic** - Nil pointer or invalid state
4. **Subsequent checks depend on this** - Cascading failures would obscure the root cause


### ✅ Example: Setup Failure

```go
func TestTaskRunReconcile(t *testing.T) {
    clients, err := test.NewClients(kubeconfig, cluster, namespace)
    if err != nil {
        t.Fatalf("failed to create test clients: %v", err)
    }
    // Test continues - clients are guaranteed to be valid
}
```

**Why?** Without clients, every subsequent operation will fail or panic.


## When to Use `t.Errorf()`

Continue testing to **collect all failures** in one run.

### Rule: Keep Going If...

1. **Checking multiple independent properties** - Each check is valuable
2. **Validating different fields** - Want to see all incorrect values
3. **Testing multiple scenarios** - One failure shouldn't hide others


### ✅ Example: Multiple Independent Checks

```go
func TestTaskRunStatus(t *testing.T) {
    tr := getTaskRun(t)
    
    // Check multiple properties - want to see all failures
    if tr.Status.PodName == "" {
        t.Errorf("expected PodName to be set, got empty string")
    }
    
    if tr.Status.StartTime == nil {
        t.Errorf("expected StartTime to be set, got nil")
    }
    
    if len(tr.Status.Steps) != 3 {
        t.Errorf("expected 3 steps, got %d", len(tr.Status.Steps))
    }
}
```

**Why?** Each check is independent. Seeing all failures helps fix them faster.



## Helper Functions: Best Practices

Helper functions are commonly used for **test setup and must-succeed operations**.

### ✅ Preferred Pattern: Use `t.Fatalf()` in Helpers

In most cases, helper functions should call `t.Fatalf()` directly instead of returning errors.

This is the idiomatic Go pattern and aligns with both the
Google Go Style Guide and Tekton’s existing conventions (e.g., `MustParse*` helpers).

### ✅ Example: Helper Using `t.Fatalf()`

```go
func mustCreateTaskRun(t *testing.T, name string) *v1.TaskRun {
    t.Helper()

    tr := &v1.TaskRun{
        ObjectMeta: metav1.ObjectMeta{Name: name},
    }

    created, err := clients.TektonClient.TaskRuns("default").Create(ctx, tr, metav1.CreateOptions{})
    if err != nil {
        t.Fatalf("failed to create TaskRun: %v", err)
    }

    return created
}
```

**Why?**

* These helpers perform setup or required operations
* Failing fast simplifies test code
* Avoids repetitive error handling in every test

---

## ⚠️ Important Exception: Goroutines

Do **not** call `t.Fatalf()` inside a goroutine:

```go
go func() {
    t.Fatalf("this will panic, not fail the test correctly")
}()
```

**Why?**
`t.Fatalf()` must be called from the test’s main goroutine.
Calling it inside another goroutine causes a **panic**, not a proper test failure.

---

## When to Return Errors from Helpers

Returning errors from helpers is appropriate only when:

* The helper runs inside a goroutine
* The caller needs explicit control over failure handling

### ✅ Example: Goroutine-safe Helper

```go
func createTaskRun(name string) (*v1.TaskRun, error) {
    tr := &v1.TaskRun{
        ObjectMeta: metav1.ObjectMeta{Name: name},
    }

    created, err := clients.TektonClient.TaskRuns("default").Create(ctx, tr, metav1.CreateOptions{})
    if err != nil {
        return nil, fmt.Errorf("failed to create TaskRun: %w", err)
    }

    return created, nil
}
```



## Summary

| Scenario                    | Use            | Reason                   |
| --------------------------- | -------------- | ------------------------ |
| Test setup fails            | `t.Fatalf()`   | Cannot proceed safely    |
| Critical preconditions fail | `t.Fatalf()`   | Avoid invalid test state |
| Multiple independent checks | `t.Errorf()`   | Report all failures      |
| Assertions                  | `t.Errorf()`   | Continue test execution  |
| Helper functions (default)  | `t.Fatalf()`   | Idiomatic and simpler    |
| Helpers in goroutines       | Return `error` | `t.Fatalf()` is unsafe   |

**Golden Rule:** Use `t.Fatalf()` when continuing the test is impossible or meaningless. Use `t.Errorf()` when you want to collect and report multiple failures in a single run.

---

## References

- https://google.github.io/styleguide/go/best-practices#test-helper-error-handling
- https://google.github.io/styleguide/go/best-practices#t-fatal
- https://google.github.io/styleguide/go/decisions#keep-going
- https://pkg.go.dev/testing