# Task 2.1: Server Map and Initialization

## Description
Initialize the `Server` state including maps, mutexes, and channels.

## Acceptance Criteria
- [ ] Struct `Server` is defined.
- [ ] `NewServer()` correctly initializes `clients` map.
- [ ] `NewServer()` initializes the `messages` buffered channel.
- [ ] Test server instances don't cause panics when locks are acquired.
