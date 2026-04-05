# Task 3.1: Message Broadcast and History Appending

## Description
The core broadcaster goroutine logic to distribute messages from `s.messages` to all clients.

## Acceptance Criteria
- [ ] Implement `broadcaster()` loop.
- [ ] Test that messages pushed to `s.messages` are fanned out to all active `c.out` channels.
- [ ] Ensure messages distributed are correctly appended to the `Server.history` slice.
