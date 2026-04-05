# Task 4.2: Client Reader Goroutine

## Description
Read scanner bytes from `conn.Read()` and push messages to `server.messages`.

## Acceptance Criteria
- [ ] Discard empty inputs.
- [ ] Send formatted structs to `server.messages`.
- [ ] Test standard EOF triggers a clean disconnection.
