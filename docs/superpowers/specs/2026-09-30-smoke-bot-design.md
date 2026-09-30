# The first bot: a smoke test

Status: design, approved in conversation 2026-09-30. Not yet implemented.

## Goal

ROADMAP.md, "Then: content and bots": bots are players with a telnet socket. They have
three uses -- a smoke test for every deploy, a load test, and a way to make an empty world
feel inhabited -- and this is the first: **after every deploy, a bot logs in to
production, walks to the millpond, fights a goose, loots it and quits, and the deploy says
whether that worked.**

The same scenario runs in `go test` against an in-process server with the real
`content/`, so a content change that breaks the walk (a renamed room, a moved goose)
fails `make check` before it is released, not after it is deployed.

Success looks like: `deploy.sh` ends with a pass or a loud fail and a transcript;
`make check` covers the walk; and nothing the bot does can mint a name on production or
litter the world.

## Standing rules

- **The bot is a player.** Plain TCP, text in and text out. It never imports `event/`,
  `command/` or `world/`, and has no side channel into the server: it tests what a player
  sees, and when the rendered text changes, it is the bot that should notice.
- **Never create characters on production** -- names are permanent. The bot logs in to a
  character that already exists, and fails if it doesn't.
- **Never roll back automatically.** A rollback disconnects everyone a second time; that
  is a person's decision.

## Health between runs

The character loses a few hp to geese every deploy. The `regen` pulse (`world/regen.go`,
5% of max every 5s out of a fight) has it back to full long before the next one, so the
bot needs no healing of its own and no wizard powers.

## Scope

In:

- `bot/`: the client and the smoke scenario
- `cmd/watchmud-bot/`: the binary
- `bot/smoke_test.go`: the scenario against an in-process server
- a configurable tick interval on `GameServer`, and an exported way to serve telnet on a
  listener the caller made, both for that test
- the bot in the image, and `deploy.sh` running it after a deploy

Out, deliberately:

- load testing, inhabitant bots, any behaviour beyond the one scenario
- the TLS port (a flag later)
- automatic rollback
- creating characters, anywhere but the test

## Package `bot/`

### `Client`

```go
func Dial(ctx context.Context, addr string) (*Client, error)  // retries until ctx is done
func (c *Client) Send(line string) error
func (c *Client) Expect(pattern string, timeout time.Duration) ([]string, error)
func (c *Client) ExpectClosed(timeout time.Duration) error
func (c *Client) Transcript() string
func (c *Client) Close() error
```

- **`Dial` retries.** `deploy.sh` runs the bot the moment `compose up` returns, while the
  server may still be loading content. It retries the connection every second until its
  context expires (30s from the binary).
- **A reader goroutine** appends everything received to a buffer, after stripping telnet
  `IAC` sequences (the server turns echo off at the password prompt). The stripping is
  the bot's own small filter, not an import of `telnet/`: the bot is a client, and
  `telnet.iacFilter` is unexported server code.
- **`Expect` waits for a regexp** to match text received since the last match, consumes
  through the end of the match, and returns the submatches. On timeout its error carries
  the last ~2KB received, so a failure reads as "wanted X, got Y".
- **Prompts are ignored.** The server writes a prompt after combat rounds nobody typed
  anything for, so "wait for the next prompt" is not "the reply has arrived". Waiting for
  the words expected is what a person does.
- **`Transcript`** is everything sent (prefixed `> `, passwords shown as `> ******`) and
  received, in order. The binary prints it on failure.

### `Smoke`

```go
type Config struct { Name, Password string }
type Result struct { Notes []string }  // things not tested, e.g. "no goose"
func Smoke(c *Client, cfg Config, log io.Writer) (Result, error)
```

Plain Go steps, each printing one line with how long it took, so a slow step shows even
on a pass. The route and the text are what content and `render.go` produce today:

1. **Banner**: expect `Welcome to WatchMUD`.
2. **Log in**: answer the name prompt, then `Password: `, then expect the room
   description. If the answer to the name is `No one by the name of`, fail with "create
   <name> by hand first" and send nothing more -- in particular never `y`.
3. **`recall`**: expect `Temple Square`. The character logs back in wherever it last
   quit, so every run starts from a known room.
4. **Walk**: `south` five times and `west` once, expecting each room's name in turn --
   Market Square, South Gate, Outside South Gate, southern path, The Waystone, The
   Millpond.
5. **Fight**: geese are aggressive and start the fight themselves. If the millpond
   description has no `An angry goose lowers its neck` line (a player killed them and the
   zone hasn't reset), record the note "no goose, fight not tested" and skip to 7.
   Otherwise count those lines -- the zone keeps up to two -- and wait for
   `angry goose is dead!` that many times.
6. **Loot**: `get all from corpse`, accepting either something taken or an empty corpse
   (the feather is a 60% drop). Then `drop all.feather` -- only what was looted. Never
   `drop all`: the character carries a waterskin from creation and would leave one at
   the millpond every deploy.
7. **`quit`**: expect the server to close the connection.

A pass with notes is still a pass. What a player did to the world shouldn't fail a good
deploy.

### `cmd/watchmud-bot`

Flags `-addr` (default `localhost:4000`) and `-name` (default `Tester`). The password is
`WATCHMUD_BOT_PASSWORD` and only that -- never a flag, so it is never in `ps` or shell
history. Exit 0 on a pass (notes printed), 1 on a failure (error and transcript
printed), 2 on bad usage (no password).

## Running it in `go test`: `bot/smoke_test.go`

- Builds the real server in-process: `loader.LoadContent(os.DirFS("../content"))`,
  `memstore`, `world.New`, `server.New`, with telnet serving on `127.0.0.1:0`.
- **Two small exports this needs**, neither changing behaviour:
  - telnet: a way to serve on a `net.Listener` the caller made (the telnet tests already
    do this internally; `Listen` becomes a wrapper around it).
  - `GameServer`: the tick interval as a field, defaulting to `rules.PulseInterval`. The
    test sets it to ~10ms. Pulse work is counted in ticks, so aggro, rounds and resets
    all speed up together, and the fight takes well under a second instead of 20-30.
- **Creates the character over the socket** first, answering the creation prompts from
  the test file (name, `y`, password twice, a lineage), then quits and runs `Smoke` as a
  fresh connection. Creation code stays out of `bot/` proper, so the binary can't reach
  it.
- A fresh world always has a goose, so this run always covers the fight; the test fails
  if `Smoke` reports the "no goose" note.
- Content is loaded from the real `content/`, not `testcontent/`: catching a content
  change that breaks the walk is the point.

## On deploy

- **Dockerfile**: builds `./cmd/watchmud-bot` into `/app/watchmud-bot` beside the game.
  Static, like the server; a few MB.
- **`deploy.sh`**, after `compose up -d`:

  ```sh
  $compose run --rm --no-deps --entrypoint /app/watchmud-bot \
    -e WATCHMUD_BOT_PASSWORD watchmud -addr watchmud:4000 -name "$bot_name"
  ```

  The same image as the version just deployed, on the compose network; `run` publishes
  no ports. `WATCHMUD_BOT_PASSWORD` and `WATCHMUD_BOT_NAME` (default `Tester`) come from
  `deploy/.env`.
- **No `WATCHMUD_BOT_PASSWORD` in `.env`: skip, with a warning.** This change can then
  ship before the character exists.
- **On failure**: print the transcript, then
  `deploy: vX.Y.Z is running but the smoke test FAILED`, and exit non-zero. The new
  version stays up; rolling back is `deploy.sh <previous>`, by hand.
- `deploy/README.md` gains: creating the bot's character by hand (once, over telnet),
  the two `.env` keys, and what a failed smoke test does and doesn't do.

## Order of work

1. The bot, the test, the image and `deploy.sh`, as above -- released like anything else.
2. By hand, on production: create the character, add the password to `deploy/.env`.
   The next deploy is the first one checked.

## Docs

- ROADMAP.md, "Then: content and bots": the smoke bot done, the other two uses still open.
- CLAUDE.md: `bot/` under Architecture (the client rule: a player, not a peer), and
  `go test ./bot` in Commands.
