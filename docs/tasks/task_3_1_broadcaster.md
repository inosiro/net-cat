# Task 3.1: Message Broadcast and History Appending

## Description
The core broadcaster goroutine logic to distribute messages from `s.messages` to all clients.

## Acceptance Criteria
- [x] Implement `broadcaster()` loop.
- [x] Test that messages pushed to `s.messages` are fanned out to all active `c.out` channels.
- [x] Ensure messages distributed are correctly appended to the `Server.history` slice.
