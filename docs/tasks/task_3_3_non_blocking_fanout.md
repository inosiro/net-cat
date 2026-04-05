# Task 3.3: Non-blocking Fan-out & Slow Clients

## Description
Ensure that a single slow-reading client does not block the entire broadcast goroutine.

## Acceptance Criteria
- [ ] Implement `select` block inside the broadcaster with a `default` case.
- [ ] Test that a blocked `c.out` channel triggers `go s.DisconnectClient(c)`.
- [ ] Ensure no deadlock occurs across the entire map of clients.
