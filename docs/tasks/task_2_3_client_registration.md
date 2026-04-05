# Task 2.3: Client Registration and History Sync

## Description
Clients must be added to the concurrent server map and immediately sent the message history.

## Acceptance Criteria
- [ ] Implement logic to add new `Client` pointers to `s.clients`.
- [ ] Ensure adding duplicate usernames is rejected.
- [ ] Implement sending current `s.history` items to the `client.out` channel.
