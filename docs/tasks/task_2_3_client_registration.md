# Task 2.3: Client Registration and History Sync

## Description
Clients must be added to the concurrent server map and immediately sent the message history.

## Acceptance Criteria
- [x] Implement logic to add new `Client` pointers to `s.clients`.
- [x] Ensure adding duplicate usernames is rejected.
- [x] Implement sending current `s.history` items to the `client.out` channel.
