# Task 4.1: Client Writer Goroutine

## Description
Implement the outgoing writer loop to send strings from `client.out` to the TCP connection.

## Acceptance Criteria
- [x] Loop over `client.out` messages.
- [x] Write bytes with a terminating `\n` to `conn.Write()`.
- [x] Test cleanly stopping when `client.out` is closed.
