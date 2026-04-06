# Task 3.3: Non-blocking Fan-out & Slow Clients

## Description
Ensure that a single slow-reading client does not block the entire broadcast goroutine.

## Acceptance Criteria
- [x] Implement `select` block inside the broadcaster with a `default` case.
- [x] Test that a blocked `c.out` channel triggers `go s.DisconnectClient(c)`.
- [x] Ensure no deadlock occurs across the entire map of clients.
