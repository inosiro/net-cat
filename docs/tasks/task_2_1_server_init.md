# Task 2.1: Server Map and Initialization

## Description
Initialize the `Server` state including maps, mutexes, and channels.

## Acceptance Criteria
- [x] Struct `Server` is defined.
- [x] `NewServer()` correctly initializes `clients` map.
- [x] `NewServer()` initializes the `messages` buffered channel.
- [x] Test server instances don't cause panics when locks are acquired.
