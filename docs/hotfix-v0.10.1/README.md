# A hotfix for v0.10.0, if you want one

Seven fixes from the night of 2026-10-06 for bugs in v0.10.0, as a patch series
on the `v0.10.0` tag. Built and checked in a scratch worktree: `go vet` and all of
`go test ./...` pass on the result, and `go test -race` on server, telnet,
writebehind and world (one telnet failure under load, once, not seen again in nine
runs; no telnet change here touches timing).

```
git checkout -b hotfix/v0.10.1 v0.10.0
git am docs/hotfix-v0.10.1/*.patch     # from a checkout of PR #36's branch
make check
```

then tag `v0.10.1` on that branch as `docs/working.md` describes. Releasing is yours.

What each one is (the PR's "In production today" has more):

1. Typed lines are cleaned of control bytes, and 0xFF doubled on the way out:
   `say`/`emote` could send the room a telnet command (WILL ECHO hides what they
   type) or a clear-screen.
2. The write-behind store retries a failed save on its own, and after a failure
   doesn't skip the next record as "unchanged".
3. `parseTarget` refuses `-1.knife` (a recovered panic and a stack trace per try)
   and `2.` with no name (acted on whatever was second).
4. `kill wild dog` takes the whole name, not "wild" -- which could be the boar.
5. "noPlayers" zones (the Barrow) don't reset with players in them.
6. `drop` saves the room at once, so a crash can't leave an item in two places.
7. A login is always answered (a store error left it waiting for good, holding the
   address's slot), and one whose connection hangs up while the password is checked
   doesn't come back as a ghost in the world.

Not in it, though fixed on the PR: the login lookup moved off the world goroutine
(a bigger change), and the bots' anchored patterns (they conflict with v0.10.0's
bots; a bot knocked off just reconnects).

Delete this directory once it's applied or decided against.
