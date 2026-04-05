# Task 4.1: Client Writer Goroutine

## Description
Implement the outgoing writer loop to send strings from `client.out` to the TCP connection.

## Acceptance Criteria
- [ ] Loop over `client.out` messages.
- [ ] Write bytes with a terminating `\n` to `conn.Write()`.
- [ ] Test cleanly stopping when `client.out` is closed.
