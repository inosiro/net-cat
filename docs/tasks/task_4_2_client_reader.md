# Task 4.2: Client Reader Goroutine

## Description
Read scanner bytes from `conn.Read()` and push messages to `server.messages`.

## Acceptance Criteria
- [x] Discard empty inputs.
- [x] Send formatted structs to `server.messages`.
- [x] Test standard EOF triggers a clean disconnection.
