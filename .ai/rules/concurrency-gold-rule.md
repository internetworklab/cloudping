# Concurrency Gold Rule

When writing concurrent code, or objects that were designed to be thread-safe, **DO NOT COMMUNICATE BY SHARING MEMORY; INSTEAD, SHARE MEMORY BY COMMUNICATING.**

## What This Means

- **Do not** use shared variables, global state, or mutex-protected fields to pass data between goroutines or threads.
- **Do** use channels, message passing, or similar communication primitives to synchronize and exchange data between concurrent units of work.

## Why

Sharing memory across concurrent execution contexts leads to subtle race conditions, deadlocks, and hard-to-reason-about code — even when protected by locks. Communication-based designs make data flow explicit, ownership clear, and correctness far easier to verify.

## Examples

### Avoid — sharing memory

```go
var mu sync.Mutex
var result int

go func() {
    mu.Lock()
    result = compute()
    mu.Unlock()
}()

mu.Lock()
fmt.Println(result)
mu.Unlock()
```

### Prefer — sharing memory by communicating

```go
ch := make(chan int)

go func() {
    ch <- compute()
}()

fmt.Println(<-ch)
```
