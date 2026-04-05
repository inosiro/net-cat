# Task 1.3: Safe Client Closure

## Description
Define the base `Client` structure and ensure clients can be tracked and disconnected safely without race conditions.

## Acceptance Criteria
- [x] Test closing an active client successfully triggers a cleanup state.
- [x] Test closing a client twice simultaneously (via Goroutines) does not panic (`SafeClose` behavior).
- [x] Implement `Client` struct with a `sync.Mutex` and boolean flag.
- [x] Implement `SafeClose()`.
