# Task 2.2: Max Connections Enforcement

## Description
Ensure the server rejects new connections when it reaches the concurrent maximum (10 connections).

## Acceptance Criteria
- [x] Test that adding a 10th client succeeds.
- [x] Test that adding an 11th client is rejected immediately.
- [x] Check logic is encapsulated by `s.mu.Lock()` to prevent race conditions.
