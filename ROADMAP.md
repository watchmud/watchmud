# WatchMUD: strangler-fig migration to telnet

## Status

**Live at watchmud.com** since September 25 2026 -- `telnet watchmud.com 4000`, or TLS
on 4443. See "Launch", below, for what that took and what's still open.

**Phases 0-6 are complete** (September 12 2026). The server is a working telnet MUD:
`make run`, then `telnet localhost 4000`, create a character, and play. Protobuf is gone;
the vocabulary is `command/` in and `event/` out. Classes are gone too, and lineage is
cosmetic: what a character is good at comes from the equipment they have on. Phase 7 --
real MUD-client protocol support -- is next.

## Launch: real players, this week (set 2026-09-24, launched 2026-09-25)

The short-term goal: deploy to a server, let real players in, and then grow content and
bots. Checked against the tree on 2026-09-24. In order -- the first group are blockers,
because today anyone can log in as anyone and anyone can spawn mobs.

**Launched 2026-09-25** on a DigitalOcean droplet in sfo3 (Ubuntu 24.04, 1GB, 1 vCPU,
2GB swap; resized up from 512MB, which couldn't hold docker, mongo and a Go build).
Checkout at `/srv/watchmud`, run as `deploy/README.md` says. Verified from outside:
both ports, a Let's Encrypt certificate for watchmud.com (TLS 1.3), mongo and every
other port unreachable, real client addresses in the log (so the per-address cap
works), `certbot renew --dry-run` passing, a backup written, and a wizard made.

**Blockers:**

1. ~~**Passwords.**~~ Done 2026-09-24. bcrypt runs on a goroutine and answers through
   `incomingBuffer` (`loginChecked`/`createHashed` in `server/commands.go`), since at
   ~70ms a hash on the world goroutine would stall everyone. The telnet conversation asks
   name first; the server answers a bare name with `PasswordRequired` or `NoSuchPlayer`,
   which picks the next question. Echo is off while a password is typed, three wrong
   tries hangs up, and a new password is 8+ characters, at most 72 bytes, typed twice.
   **Characters made before this have no hash and can't log in** -- no migration, on
   purpose (anyone who knew the name could claim it). Production starts from an empty
   mongo, so there is nothing to clear; just don't copy a dev database across.
2. ~~**Wizard commands are open to everyone.**~~ Done 2026-09-24. `Wizard` on the record
   (and the mongo document), set by hand with `make wizard NAME=...` while that character
   is logged out. Builder commands carry the `command.Wizard` marker and
   `World.HandleIncomingMessage` refuses them before the switch, answering a non-wizard
   exactly as it would a verb that doesn't exist. `load`, `restore` and `roomstatus`
   (which dumps room internals) are gated. A record flag rather than a names list in
   app.yaml, because anyone could create a listed name before its owner did.
3. ~~**Names.**~~ Done 2026-09-24. `player.CanonicalName`: 3-16 letters a-z, stored
   capitalized ("bOB" is Bob), so store lookups stay exact and mongo's unique index is
   case-insensitive for free; `player.List` compares by `NameKey` (lowercase), so `tell
   bob` finds Bob. `World.IsReservedName` refuses the target grammar (`all`, `self`,
   `corpse`, `someone`...) and every loaded mob's name and aliases -- asked only of names
   nobody has, so a mob added later doesn't lock out the player who had it first. Both
   are checked at the name prompt, before a password or creation is offered.
   Not reserved: object names (a player called Knife can't be picked up -- `get` never
   searches players), and look-alike names (Bob vs Bobb). Neither is worth it yet.
4. ~~**Connection hygiene.**~~ Done 2026-09-24.
   - **Panics:** `GameServer.recovering` wraps every dispatch -- the stack goes to the log,
     the player gets "Something went wrong", a connection mid-login gets a failed login
     rather than waiting forever. Each heartbeat job recovers on its own
     (`recoverPulse`), so a crash in combat doesn't also skip the save after it. The
     world can be left half-changed by a handler that died partway; still better than
     no world.
   - **Idle:** a read deadline before every line, 2 minutes at login and 30 in game
     (`defaultLoginIdle`/`defaultPlayIdle` in `telnet/conn.go`). Timing out in game is
     a logout with cause `idle`: saved, out of the world.
   - **Per address:** 5 connections per IP (`maxConnsPerAddress`), refused with a
     message past that. Constants for now; move them to app.yaml if they need tuning
     without a build.
5. ~~**`help`.**~~ Done 2026-09-25. `help`, `?` or `commands`, answered by the connection
   (`telnet/help.go`) rather than the world, since it lists typed verbs and only
   `parse.go` knows those. Each entry names its verbs and `help_test.go` parses every
   one, so help can't offer a verb the parser refuses, or a builder command. "help" at
   the name prompt explains the prompt instead of creating a character called Help.
   `shout` is now an alias for `tellall`, which already rendered as a shout.
6. ~~**Deploy.**~~ Done 2026-09-25, TLS included. `Dockerfile` (static binary on distroless,
   non-root, 12.5MB) and `deploy/compose.yaml`: the game, mongo *with auth* and no
   published port, and a backup service (nightly `mongodump`, two weeks kept, into
   `deploy/backups`). Logs are stdout (an empty `log.file`) rotated by docker. The
   mongo uri, password and all, comes from `WATCHMUD_MONGO_URI` rather than the
   checked-in `deploy/app.yaml`, and is logged redacted (`mongostore.RedactURI` -- it
   was logging the password). `stop_grace_period: 30s` so SIGTERM's flush of queued
   saves isn't cut off. The telnet bind is `telnet.host` in config (54c319d). Runbook:
   `deploy/README.md`. Tested end to end locally, including a restart with a player
   connected (they came back where they were) and restoring a backup.

   **TLS** is port 4443, the same game, in Go (`telnet/tls.go`): certbot on the host,
   `deploy/certbot-hook.sh` copies each renewal where the non-root container can read
   it, and the server reloads it on the next handshake -- a renewal restarts nothing and
   disconnects no one. Handshakes run on their own goroutine with a 10s deadline, so a
   plain telnet client on the TLS port can't hold up accepts or keep its slot. The
   5-per-address cap is shared across both ports. The plain port's banner advertises
   4443. A listener that fails -- a port in use, a certificate that won't load at
   startup -- now stops the server instead of leaving it running with no way in.
   Tested in the compose stack: a whole session over TLS 1.3, and a renewal swapped in
   with a player connected, who stayed connected.

   Still open:
   - ~~**Copy backups off the host.**~~ Done 2026-09-25: the `offsite` service (rclone)
     copies each dump hourly to a Spaces bucket in another region, which keeps 90 days;
     `copy`, never `sync`, so the droplet pruning its own doesn't prune the bucket.
     The bucket key can delete (that's how pruning works), so a compromised droplet
     could empty it -- so the droplet's own DigitalOcean backups are on too (added
     2026-09-25), a copy nothing on the droplet can delete. The layers, and what each
     survives, are a table in deploy/README.md. Possible later: if Spaces offers a key
     that can write but not delete, prune with a bucket lifecycle rule instead and
     give the droplet that key.
   - ~~**Build the image in GitHub Actions.**~~ Done 2026-09-26.
     `.github/workflows/build.yaml` tests every push and PR (make check, the mongo tests
     against a real mongo, compose.yaml's validity) and pushes images to
     `ghcr.io/watchmud/watchmud`: `:master`, `:release-X.Y`, `:sha-...` for development,
     and `:vX.Y.Z` from a version tag, which must be on a `release/` or `hotfix/` branch.
     Production runs only versions, moved by hand with `deploy/deploy.sh vX.Y.Z` -- which
     checks out the tag (compose.yaml and scripts are versioned too), pulls, and only
     then restarts, putting everything back if a step fails. Content is in the image,
     so a version is code and world together. The binary logs its version at startup.
     Branching (master / release / hotfix) is in deploy/README.md, "Releasing".
     Later, maybe: a manual "deploy" workflow that runs deploy.sh over ssh, with a key
     that can run nothing else.
   - **watchmud.games** has DNS but points nowhere. An A record and `-d watchmud.games`
     on the certificate, if it should.

   Why TLS was done this way, for the record: **a TLS port** beside the plain one, so a
   password doesn't have to cross the internet in the clear. Telnet has no encryption and no MUD protocol adds any (MCCP
   is compression; GMCP/MSDP/MTTS are data channels; telnet START_TLS, option 46, has
   almost no client support), so what the better-run games do is a second port that
   speaks TLS. Mudlet has a "Secure" checkbox, TinTin++ has `#ssl`, and anything else
   can use `openssl s_client -connect host:port`. Two ways to do it:
   - In Go: `telnet.Listen` has one `net.Listen` (`telnet/conn.go`); wrap it in
     `tls.NewListener` for the second port. Everything above the socket -- `iacFilter`,
     login, the pumps -- works on a `net.Conn` and doesn't change. Needs the cert and
     key paths in config (both in `serverconfig`, it's `UnmarshalStrict`), and a
     renewal story: certbot on the box, reload on renew.
   - Or terminate TLS in a proxy (Caddy's layer-4 module, haproxy, stunnel) that
     forwards plaintext to 4000. No Go at all, one more moving part -- and **every TLS
     player then arrives from the proxy's address**, so the 5-per-address cap refuses
     the sixth. PROXY protocol, or exempting the proxy, fixes that. TLS in Go doesn't
     have the problem, which makes it the better of the two.

   Keep plain telnet: stock `telnet` can't speak TLS, and locking those players out
   costs more than it protects. Say at the login banner that the secure port exists.

**First week, once people are in:**

- ~~A welcome line after login pointing new characters south to the Hollowfields.~~ Done
  2026-09-28: `"welcome"` in settings.json, said once, at creation.
- ~~Bare `corpse` should mean the newest corpse, not the oldest (found in play).~~ Done
  2026-09-28: a room's floor lists newest first (`object.NewFloor`).
- The Barrow-King's AC call (LEVELS.md).
- ~~Mob taunts, the first Lua (below, "No scripting language").~~ Done 2026-09-28:
  `script/`, two fight hooks and `me:say`; the Barrow-King has a script.

**Then: content and bots.** Bots are players with a telnet socket: a small Go client
that logs in and walks, kills, loots and talks. The same harness is a load test, a smoke
test for every deploy (the one used by hand on 2026-09-24 walked Wrathrock to the
throne room and looted a goose), and a way to make an empty world feel inhabited.

- ~~**The smoke test.**~~ Done 2026-09-30: `bot/` (a telnet client and one scenario) and
  `cmd/watchmud-bot`, run by `deploy.sh` after every restart and by `go test` against
  the real content in-process. Load testing is still open; it builds on `bot.Client`.
- ~~**Inhabitants.**~~ Done 2026-09-30: `bot.Adventurer` and `cmd/watchmud-bots`, the
  `bots` compose service -- always-on, labelled `[bot]` in `who` from a flag on the
  record, hunting the Hollowfields, donating what they find, and honest when told
  to. Dropped items decay after 30 minutes, so the donation room turns over.
  Future: a **Wanderer** (roams, fights only when attacked, never loots), a
  **Socialite** (greets new characters in town, answers newbie questions), bots that
  **explore and map** the world instead of following hand-written hunting grounds
  (needs room names to stay unique -- true today, not enforced), and bots that **wear
  the upgrades they find** and grow into the Barrow.

The Context section below describes the tree as it was in September 2026, before any of
this landed. It is kept for its reasoning, not as a description of the present.

## Later: the Kubernetes cluster?

There's a hosted cluster already running other projects. The compose deploy was built
so this stays open: same image, config in a file, secrets in the environment. Nothing
needs undoing to move. What's different about this service, and what it would take.

**It is a singleton, and must stay one.** One process owns the world: every room,
every fight, who is where. Two copies aren't two servers, they're two worlds, each
saving the same characters over the other -- which is how items get duplicated. So:
`replicas: 1` and `strategy: Recreate` (the default rolling update starts the new pod
before stopping the old one, which is exactly the two-copies case), or a StatefulSet.
No horizontal scaling, ever, without a real redesign.

**Every restart is an outage.** Players hold a socket for hours; a new pod means every
one of them drops and reconnects. k8s restarts pods more often than a VPS restarts
containers: node upgrades, drains, rebalancing, eviction under memory pressure. A
PodDisruptionBudget can't help a singleton -- `minAvailable: 1` just blocks the drain.
`terminationGracePeriodSeconds: 30` so the save flush finishes, and it would be worth
announcing a shutdown to players from SIGTERM first.

**Raw TCP in.** Ingress controllers are HTTP. Telnet needs a `Service` of type
`LoadBalancer` (usually one paid cloud load balancer per service), or the ingress
controller's TCP passthrough, or a Gateway API `TCPRoute`. Two things to check on
whichever it is:
- **Idle timeouts.** Cloud load balancers drop idle TCP connections (often 350s to a
  few minutes); a player reading a room description counts as idle. Go's accepted
  connections send TCP keepalives every 15s by default, which usually covers it --
  verify, don't assume.
- **Client addresses.** A load balancer that rewrites the source address makes every
  player the same address to the 5-per-address cap. `externalTrafficPolicy: Local`,
  or PROXY protocol.

**Mongo.** In the cluster it's a StatefulSet with a PersistentVolumeClaim -- the volume
is tied to one zone, so the pod can only reschedule there -- and backups become a
CronJob pushing `mongodump` to object storage (off-host, which the VPS still lacks). Or
a managed mongo outside the cluster, which moves the whole problem elsewhere.

**Health checks need an endpoint first.** A TCP probe would open a telnet session every
few seconds, and wouldn't notice the one failure that matters: the world goroutine
wedged while the listener still accepts. The fix is a small HTTP endpoint (`webPort` is
still in the config struct, unused) reporting the time of the last heartbeat, failing
when it's stale. Worth having on the VPS too.

**For it:** restarts, rescheduling and health checks done for you; secrets, logs and
monitoring the other projects already use; rolling out a new image is one command;
cert-manager for the TLS certificate; the cluster is already paid for.

**Against it:** a stateful singleton that holds long connections uses almost none of
that, and each piece above is something to get right that a VPS doesn't ask for.
Rescheduling -- the main thing k8s adds -- is an outage for this service.

**Recommendation:** launch on compose. Move when there's a reason: the VPS becoming a
second thing to maintain, or wanting the cluster's monitoring. The health endpoint is
worth doing either way, and is the first step if it moves.

## Context

This started as a learn-Go project: gRPC/protobuf transport, a separate console client
(`trasa/watchmud-client`), and a Postgres database. The destination is different — a real
telnet server that tintin++ and other MUD clients can connect to, no custom client, and
some persistence layer that isn't the current SQL schema.

The current tree is further from working than it looks. `cmd/watchmud/main.go` builds the
world and runs the tick loop, but **nothing calls `rpc.NewServer`, `web.Start`, or
`db.Init`** — there is no listener and `watchdb` is a nil handle, so the first thing a
connection would do (login → `db.GetPlayerData`) fails. `make vet` has never been green
(copylocks, from passing protobuf structs by value), one `playergenerator` test fails, and
the race→lineage/species rework is half-landed with `PLACEHOLDER` values in
`server.handleCreatePlayer` and commented-out DB calls in `world/h_stat.go`.

The good news: **the strangler-fig seam already exists.** `client.Client` (Send / SetPlayer /
GetPlayer / Close) and `gameserver.Instance` (Receive / Logout) already abstract the
transport. `rpc/client.go` is one implementation. Telnet is a second one. Nothing in
`world/` needs to know which is talking.

Decisions taken up front:
- **Telnet is the first live listener.** `rpc/` and `web/` stay on disk, unwired, as
  reference; they get deleted once telnet covers their ground. **DONE** -- both deleted
  September 2026, along with `client/`.
- **Persistence becomes an interface with an in-memory implementation.** The `db` package
  and Postgres go away now; the real replacement is chosen later, against a working server.
- **Protobuf stays as the internal vocabulary for now.** Telnet parses text into
  `message.XRequest` and renders `message.XResponse` back out. `world/` is untouched.
  Re-evaluated in Phase 5.
- **The player model is fixed before telnet is written**, so the telnet connection is
  written once against the final shape.

Ordering principle throughout: never be more than one small step away from a server you can
start and connect to.

---

## Phase 0 — A baseline you can trust

Nothing else is measurable until `go test ./...` means something.

- `playergenerator/generator_test.go:35` fails (expects 16, got 15) — almost certainly
  fallout from the lineage/species rework in `e09628b`. Either fix the generator or pin the
  test to current intent; don't leave it red as background noise. **DONE September 2 2026**
- `make fmt-check` silently always passes: it calls `gofmt -files`, which is not a real
  flag, and the target swallows the error. Change to `gofmt -l .`. **FIXED September 2 2026**
- **Leave `make vet` red.** Every failure is `copylocks` from protobuf structs passed by
  value (`p.Send(message.LookResponse{...})`, `resp := x.(message.LookResponse)`). These
  disappear on their own if Phase 5 removes protobuf. Chasing them now is wasted work
  against code that may not survive. **NO ACTION REQUIRED September 2 2026**

Exit: `make test` green, and you know what red means again.
All tests pass, `make fmt-check` succeeds, `make vet` still fails as expected.
**Completed** September 2 2026**

---

## Phase 1 — Collapse `player.Player` to a struct

**The problem.** `player.Player` (`player/player.go`) is a 30-method interface with exactly
two implementations: `server.ClientPlayer` and `player.TestPlayer`. One of those is a test
double. The other lives in the wrong package. This is a C#/Java habit — "program to an
interface" — where Go wants a concrete type.

**Why it ended up that way.** `client` imports `player` (its `GetPlayer`/`SetPlayer` return
`player.Player`). A concrete player that holds a `client.Client` would make `player` import
`client` — an import cycle. The interface was the escape hatch. Verified: `player/` imports
only `combat` and `object` today.

**The Go-idiomatic fix.** Define the *narrow* interface at the point of need, inside
`player`, and depend on that instead:

```go
package player

// Sender is anything that can deliver a message to this player's connection.
type Sender interface {
    Send(msg interface{}) error
}

type Player struct {
    Id        int64
    Name      string
    out       Sender          // was: client.Client, via ClientPlayer
    inventory *Inventory
    slots     *object.Slots
    // curHealth, maxHealth, dirty, location, abilities ...
}

func New(name string, out Sender, ...) *Player
func (p *Player) Send(msg interface{}) error { return p.out.Send(msg) }
```

`client.Client` satisfies `player.Sender` structurally — no import, no cycle. The future
telnet connection satisfies it too, which is the whole point of doing this before Phase 4.

Work:
- New concrete `player.Player` struct in `player/player.go`, absorbing the fields and all 30
  method bodies from `server/clientplayer.go`. **DONE**
- **Delete `server/clientplayer.go` and `player/testplayer.go`.** Replace the test double
  with a tiny `player.Recorder` implementing `Sender` and capturing sent messages; tests
  become `p := player.New("testdood", rec)` and `rec.Sent(0).(message.LookResponse)`,
  replacing today's `p.GetSentResponse(0)`. Same for `client.TestClient`, which mostly
  duplicates it. **DONE**
- Mechanical sweep `player.Player` → `*player.Player` in `spaces/` (`room.go`,
  `roominventory.go`), `world/` (`playerroommap.go`, most `h_*.go`), `gameserver/handlerparameter.go`,
  `client/client.go`. **DONE**
- `player/players.go`: `List` maps keyed on `*Player`. Its `sync.RWMutex` is vestigial —
  world state is single-goroutine by design (see the comment in `Room.CreateRoomDescription`)
  and stays that way after telnet, since connection goroutines only push to a channel. Leave
  the mutex or drop it, but don't let it imply the world is concurrent. **DONE**
- **Keep `combat.Combatant`.** That interface is real polymorphism — `*player.Player` and
  `*mobile.Instance` both fight, and `combat/melee.go` genuinely must not care which. Don't
  collapse it along with the others. **DONE**

Note after this lands: `client.Client` is now nearly redundant — `player.Sender` plus a
`Close()`. Worth deleting in Phase 4 rather than pre-emptively here.

Exit: `make test` green, `player.Player` is a struct, one fewer package in the cycle.
**DONE September 4 2026**
---

## Phase 2 — Persistence behind an interface

Files: new `player/store.go`, new `memstore/`, delete `db/`.

```go
package player

type Record struct {
    Id                          int64
    Name                        string
    CurHealth, MaxHealth        int64
    LineageId, ClassId          string   // string ids, not the int32s (see Phase 6)
    LastZoneId, LastRoomId      string
    Abilities                   Abilities
    Slots                       []SlotRecord
    Inventory                   []InventoryRecord
}

type Store interface {
    Load(name string) (*Record, bool, error)
    Create(r *Record) (*Record, error)
    Save(r *Record) error
}
```

`Record` mirrors what `db.PlayerData` + `db.PlayerInventoryData` + `db.SlotDataList` carry
today (see `db/sql/ddl.sql` for the full field list) — but **use string lineage/class ids
now**, even though nothing consumes them until Phase 6. Writing `int32` race ids into a new
store just to migrate them again is wasted motion. **DONE**

Rewire the four call sites: `server.handleLogin`, `server.handleCreatePlayer`,
`world.HandleIncomingMessage` (the save-after-every-handler), and `world/h_logout.go`. Pass
the `Store` into `server.New` and `world.New` rather than reaching for a package global —
`db.watchdb` being a package-level var is part of why none of this is testable today.
**DONE**

Then delete `db/` and drop `sqlx`, `lib/pq`, `go-sql-driver/mysql`, `mattn/go-sqlite3`, and
the `golang.org/x/crypto/ssh` tunnel from `go.mod`; strip `DB` and `SSH` from
`serverconfig.Config` and `app.local.yaml`. **DONE**

`memstore` is a `map[string]*player.Record` behind the interface. Players don't survive
restart. That is fine and it unblocks login today. **DONE**

One thing to notice while you're here: `HandleIncomingMessage` saves the player after
*every* message. Free against a map, absurd against a real store. Leave it; the interface
means the eventual implementation can batch or debounce without touching `world/`.

Exit: no SQL in the tree, login path runs without a database.
**DONE September 7 2026**
---

## Phase 3 — Make the loop answer typed input promptly

`server/gameserver.go`: `incomingBuffer` is unbuffered and is only drained *inside*
`heartbeat`, which runs on the 1s `mudtime.PulseInterval` ticker. A telnet user would wait
up to a full second between pressing enter and seeing output. That is the difference between
a MUD that feels alive and one that feels broken, and you want it fixed before your first
telnet session forms an impression.

Restructure `Run` to select on both:

```go
for {
    select {
    case <-ctx.Done():
        return ctx.Err()
    case msg := <-gs.incomingBuffer:
        gs.dispatch(msg)          // handle immediately
    case <-ticker.C:
        pulse++
        gs.heartbeat(pulse, delta) // zone/mobile/violence pulses only
    }
}
```

Move the message-draining loop out of `heartbeat`, leaving it purely for pulse work. This
preserves the single-goroutine invariant exactly — commands are still processed one at a
time, never concurrently with a pulse. Give `incomingBuffer` a modest buffer (say 64) so a
burst of input doesn't block connection goroutines.

Exit: command latency is bounded by processing, not by the tick.
**DONE September 7 2026**
---

## Phase 4 — The telnet listener

New package `telnet/`. Read `rpc/client.go` first — it is the template for what you're
writing, and `readPump`/`writePump` map almost directly.

**Connection** — `telnet.conn` implements `player.Sender` and holds a `net.Conn`:
- `Listen(addr string, gs gameserver.Instance)`: accept loop, goroutine per connection.
- Read side: `bufio.Scanner`/`Reader` over lines, strip `\r\n`, then parse → `message.XRequest`
  → `message.NewGameMessage` → `gs.Receive(gameserver.NewHandlerParameter(c, gm))`.
- Write side: keep the `sendQueue chan` + write-pump shape from `rpc/client.go`; it already
  handles the "world goroutine must never block on a slow socket" problem.
- On read error/EOF: `gs.Logout(c, cause)`, same as `rpc`.
- At minimum, strip `IAC` (0xFF) sequences from input now so a real client's opening
  negotiation doesn't get parsed as a command. Full negotiation is Phase 7.

**Login state machine** — before the connection is in the world, it isn't yet a player. A
small per-connection state (`awaitingName` → `awaitingConfirmNew` → `inGame`) that emits
`message.LoginRequest` / `message.CreatePlayerRequest` into the same pipe. Everything
downstream is unchanged.

**Command table** — new, and the piece with no existing analogue in this repo (it lived in
the client). Verb, aliases, and minimum abbreviation, mapping to a request constructor:

```go
{"north", []string{"n"}, func(args string) any { return message.MoveRequest{Direction: direction.North} }},
{"look",  []string{"l"}, ...},
{"get",   nil,           func(args string) any { return message.GetRequest{Target: args} }},
```

**Do not parse targets here.** `world.parseTarget` (`world/target_parser.go`) already handles
`all`, `all.knife`, `2.knife`, `20 coins`, and it is exercised by
`world/target_parser_test.go`. Pass the raw argument string through in the request and let
the server parse it, exactly as the gRPC client does today. Duplicating that grammar in the
transport is how the two drift apart.

**Renderer** — the substantial new work: one type switch over every `message.*Response` and
`message.*Notification`, producing text. `spaces.Room.CreateRoomDescription` already returns
a structured `message.RoomDescription` (name, description, exits, players, objects, mobs) —
the renderer turns that into the classic room block. This type switch is the single
chokepoint that Phase 5 would rewrite, which is why it is worth keeping it in one file and
free of game logic.

Wire it in `cmd/watchmud/main.go` alongside the existing setup, on a new `telnetPort` in
`app.local.yaml`. Delete `client/` (the interface serving the dead console client) and
`client.TestClient` once nothing references them.

Exit: `telnet localhost 4000`, log in, `look`, `north`, `get knife`, `inventory`, `who`, `quit`.
**DONE September 11 2026.** `telnet/` is `conn.go`, `protocol.go`, `render.go`,
`resultcode.go`, with tests for the renderer and the IAC filter.

What actually happened, where it differed from the plan above:

- **`client.Client` didn't just get deleted, it moved and got a setter.** It became
  `gameserver.Conn` (`player.Sender` + `Player`/`SetPlayer`/`Close`), next to `Instance`
  and `HandlerParameter` in the package that already is the transport seam. Two bugs fell
  out first, in their own commit: `gameserver.Instance.Logout` took `*client.Client` --
  pointer to an interface -- so `*server.GameServer` had never satisfied `Instance`; and
  nothing could attach a player to a connection, so `handleLogin`'s `msg.Player = p` was
  discarded and every command after login would have seen a nil player.
- **`player.Sender.Send` lost its `error` return.** 87 call sites, 5 used the error, and
  the only error any implementation could return meant "this connection is already dead
  and I already tore it down." Deleting it also deleted three `// TODO error handling`
  comments in `spaces/room.go` that were never going to be resolved.
- **The command table was never written.** `message.TranslateLineToMessage` already
  existed in `watchmud-message` (written for the dead console client) and covers 20 of the
  21 handlers, aliases included. `telnet` owns only `quit` -- which has no request type of
  its own and becomes a `LogoutRequest` -- plus a bounds guard for `drop`, because
  `translator.go` indexes `tokens[1]` unguarded and a bare `drop` panics the process.
- **The login state machine is straight-line code, not a state field.** A login is a
  sequential conversation, so it wants a stack, not a state machine: `login()` runs on the
  read goroutine and blocks on an `authResult` channel that `Send` signals when a
  `LoginResponse` or `CreatePlayerResponse` goes past. The connection mutex ended up
  guarding only `player`.
- **The renderer is two files, not one.** `render.go` is keyed on protobuf types and is
  what Phase 5 retargets; `resultcode.go` maps the `ResultCode` strings `world/` emits to
  player-facing text and survives Phase 5 untouched. `TARGET_NOT_FOUND` is used by six
  handlers with two different meanings ("not here" vs "not carrying it"), so the table
  takes a verb as well as a code.
- **"Don't parse targets in the transport" was half right.** True for `drop` and `equip`,
  which take a raw string and call `world.parseTarget`. False for `get`, which reads
  `FindMode`/`Index`/`Target` and expects the *client* to have parsed them. Two grammars
  live in the tree; the translator happens to satisfy both. Phase 5 resolves it.
- **`kill` was blocked by something unrelated.** `h_kill.go`'s `fightLedger.Fight` call was
  commented out because it did not compile: after Phase 1 collapsed `Player` to a struct,
  `*player.Player` implemented exactly *one* of `combat.Combatant`'s ten methods, and
  nothing caught it because nothing ever assigned one to the other. Fixed in this phase --
  see the combat notes below.
- **Two latent world bugs surfaced during manual testing.** `World.RemovePlayer` never
  removed the player from the `Room`'s own list, so quitting left a ghost; and
  `RoomInventory.GetAll` iterated a map, so a room's contents shuffled on every `look`.
  (The second one turned out to have four more instances -- a room's mobs, the players in
  a room, a player's inventory and the `equipment` listing. All of them are `ordered.List`
  now; see CLAUDE.md "Definition vs Instance".)

Combat was repaired here rather than deferred, since `kill` is in the exit criteria:
`Combatant` split into `Attacker` and `Defender` (the roles in a single swing, which swap
every round) plus `Combatant` (the entity that persists across them, holding `Id`,
`Dead` and `TakeMeleeDamage`). `Type() CombatantType` and its enum are gone -- it existed
so callers could un-abstract, and `corpse.go` now does an honest type switch. `FightLedger`
is keyed on `uuid.UUID` instead of on the interface itself.

That made combat *compile and cohere*, not work. The violence pulse runs and the melee math
is correct, but nothing drives a fight -- no aggression, nothing that sustains or resolves
one in play. Treat combat as structurally sound and behaviourally absent.

---

## Phase 5 — Decide protobuf's fate

**Decided: it goes. DONE September 12 2026.** Telnet was the only consumer, so the
`GameMessage` oneof wrapper and `DecodeTypeName` string-keyed dispatch were pure overhead,
and every `copylocks` finding from Phase 0 was a symptom of value-copying types that were
never meant to be copied.

`command/` (in) and `event/` (out) are plain Go structs. `make check` — fmt-check, vet and
test — is green for the first time in the project's history, which is the signal the
value-copying is actually gone rather than hidden.

Where it differed from the sketch above:

- **Three shape decisions mattered more than the mechanical conversion**, and were taken
  before the structs were written rather than discovered afterwards:
  - **`Success bool` + `ResultCode string` did not survive.** A success event carries its
    payload and nothing else; failure is a single `event.Failed{Verb, Code}` with a typed
    `event.ResultCode`. `render.go` had ~25 cases each opening with
    `if !m.Success { return failureText("<verb>", m.ResultCode) }` where the verb was a
    per-case constant — a table pretending to be a switch. It is now one case. The verb
    comes from `command.Command.Verb()` via `HandlerParameter.Fail`, so a handler never
    repeats its own name, and the `"PARSE_ERROR_" + err.Error()` string-splicing (which
    could put a raw `strconv` message in front of a player) is gone.
  - **Response and Notification merged** wherever the difference was only audience.
    `renderViolence` already took `self` and did exactly this. Drop, get, say, tell and
    shout became one event each with an `Actor`, and look/move/recall — which had always
    rendered identically — became one `event.RoomDescription`. Roughly half as many types,
    and response/notification can no longer drift apart.
  - **Naming is imperative in, past tense out**: `command.Drop` / `event.Dropped`, not
    `DropRequest` / `DropResponse`, which stuttered inside packages already named for the
    direction of travel.
- **`world.handlerMap` became a type switch, not a typed key.** Handlers take their command
  as a second parameter, so `msg.Message.GetDropRequest()` is gone from every one of them.
  The cost is that a type switch has no exhaustiveness check — a missing case compiles fine
  — so `world_unknownMessage_test.go` now covers the default arm.
- **`direction/` and `slot/` moved into this repo** as top-level packages. They were always
  domain types rather than wire types: `object`, `player` and `loader` imported them
  directly, with no `message` involved. `message.FindMode` was deleted outright rather than
  moved — `get` now passes a raw target string like every other command, which closes the
  "two target grammars" problem the Phase 4 notes left open.
- **The parser came home too.** `message.TranslateLineToMessage` became `telnet/parse.go`,
  returning `(command.Command, error)`. `quit` folded in as `command.Logout`, and the
  bare-`drop` bounds guard disappeared entirely: an empty target is a normal `NO_TARGET`
  failure, so the input that used to panic the process is now just a command that fails.
- **Three bugs surfaced, two of them pre-existing.** `server.dispatch` dereferenced
  `msg.Message` unconditionally and panicked the process on the first converted command
  (found by a live telnet session, not by any test — nothing in `world/`'s unit tests
  reaches `server.dispatch`). Recall rendered as "alice leaves none!." because
  `movePlayerMagically` moves with `direction.None`; that had always been true and is now
  guarded in the renderer. And `rules/species.go`'s malformed `json:"id""` tag — the one
  real vet finding hiding in the copylocks noise — is fixed.
- **`go mod tidy` dropped testify to v1.2.2**, which has no `require.Greater`. The message
  module had been raising it through MVS all along. Bumped to v1.11.1.
- **Deleted along the way**: `handleDataRequest` (nothing had emitted a `DataRequest` since
  the console client died), `combat.CombatantType` (a type with no values), and from
  `go.mod`: `watchmud-message`, grpc, protobuf, genproto, and `gorilla/mux` — the last two
  already had zero Go references, left over from the deleted `web/` and `rpc/`.

---

## Phase 6 — Lineage goes cosmetic, class goes away

**DONE September 12 2026.** The plan in this slot used to be "finish lineage/species and
give `playergenerator` the catalog so ability generation reflects actual bonuses." That
plan was thrown out before any of it was written, because the thing it was finishing was
the thing that felt stale: a lineage that hands out +2 CON, and a class picked once at
creation and carried forever.

Two decisions replaced it:

- **A lineage is cosmetic.** It grants no bonuses and carries no penalties, the way a
  gender wouldn't. Creation picks one and picks nothing else. `rules.Species` survives
  purely to group lineages in the creation menu.
- **There is no class. Equipment is the class.** Object definitions declare what they
  contribute to each role; the weights of everything equipped are summed; the highest total
  is your role. Wear armor in every slot and you are a Tank. Take it off, hold a censer, and
  you are a Healer before the next prompt.

What landed:

- `rules.Class` and `content/rules/classes.json` deleted; `rules.Role` and
  `content/rules/roles.json` (tank, healer, striker) in their place. `Catalog.RoleFor`
  resolves a weight map to a role -- highest total, ties to whichever role the content
  declared first, and **nil when the gear argues for nothing**, because "you are wearing
  nothing in particular" is a real answer and a default would hide it.
- `object.Definition.RoleWeights`, from the `"roles"` key in `objects.json`, validated
  against the catalog at load time: an unknown role id is a hard startup failure rather
  than gear that mysteriously does nothing.
- `player.Slots.RoleWeights()` sums it. Nothing stores a role -- not on `Player`, not in
  `Record` -- because a stored role can disagree with the equipment.
- **`rules.Abilities` deleted outright.** It went in stages and the first two were wrong:
  first `Add` and `Set` went with the bonuses and preferences that used them, then
  `StandardAbilities()` lost its argument so everyone started equal. At which point it was
  a struct of six numbers that every character shared and only `stat` read -- dead code
  wearing a crown, and exactly the sort of thing a later mechanic gets quietly routed
  through. `event.Stat` lost its six fields, `player.Record` lost the column, and the
  ability block is gone from `stat` output. If real combat stats arrive, derive them from
  equipment rather than resurrecting this.
- `player.Record.ClassId` is gone, and an unrecognized `LineageId` stopped being fatal:
  same reasoning as the missing-definition case next to it, since a cosmetic field is never
  worth locking someone out of their character over.
- New `stat` (which was entirely `// TODO Phase 6` placeholders and is now filled in),
  `role`, and role in `who` where a class would traditionally sit.
- Creation over telnet asks for a lineage, grouped by species, answered by number or by
  any unambiguous prefix of the name. `telnet.Listen` takes the catalog for this; the
  renderer already depended on `rules`.

Three things surfaced that the plan above didn't anticipate:

- **`remove` did not exist.** Wear and wield had no counterpart, so a character could put
  gear on and never take it off. That is survivable when a class is a permanent label and
  fatal when equipment *is* the class, so `remove` was written here rather than deferred --
  "switch eq to switch role" is not a feature you can ship half of. Found by playing it,
  not by a test.
- **`Slots.Set(loc, nil)` was a trap.** `IsSlotInUse` tested for key presence, so a nil
  left behind read as "occupied" and would have made a slot unusable for the rest of the
  character's life. `Slots.Clear` deletes the key, `IsSlotInUse` tests the value, and
  `FromRecord` now skips a slot whose item didn't survive a content edit instead of
  storing a nil there.
- **`handleShowEquipment` would have dereferenced that nil.** Pre-existing, unreachable
  until `remove` made empty slots ordinary.

Not done, deliberately: nothing in `combat/` reads a role. The melee math is fine but
nothing drives a fight (see "Known problems"), so a tank bonus would be a number no player
could observe. Roles are derived and displayed; wiring them into combat belongs with the
gameplay layer that makes fights happen at all.

---

## Phase 7 — Real MUD-client protocol support

Additive once the byte loop exists:
- TELNET option negotiation (`IAC WILL/WONT/DO/DONT`), `ECHO` off for password entry.
  ECHO done 2026-09-24 (with passwords). ~~Prompt marking~~ done 2026-10-01: `IAC GA`
  after every prompt, `IAC EOR` once the client says `DO EOR` (CLAUDE.md, telnet/).
- ~~ANSI color, with a per-player toggle.~~ Done 2026-10-01: `color [on|off]`, saved on
  the record, on by default; `telnet/ansi.go` is the palette.
- `NAWS` (window size) for wrapping; wrap output to the client's width.
- Then the MUD-specific layer as it earns its keep: `MSSP`, `GMCP`, `MCCP`, `MXP`.
- Test against tintin++ specifically, since that's the target.

---

## Known problems deliberately *not* scheduled

Named so they don't get rediscovered as surprises:

- **Dual location bookkeeping.** Not a launch item; general cleanup, and the thing Lua
  would build on, so do it before Lua. Laid out to be walked through in order.

  **Done 2026-09-26** (steps 1, 2 and 4 below): `spaces.Occupancy` holds both directions
  for players and mobs and is the only writer of a room's lists, whose writers are now
  unexported. Kept for the reasoning. Also gone with it: `movePlayerMagically` pretending a
  player who is nowhere was in the void room (and telling the void they left), and the
  `ZoneId`/`RoomId` a `Fight` used to snapshot -- a fight's room is its fighter's room.

  *What was there* (checked 2026-09-24):

  - ~~**Players, two copies.**~~ `world.playerRoomMap` (player -> room) and each `Room`'s
    `playerList` (room -> players). `World.addPlayerTo`, `RemovePlayer` and `movePlayer`
    each updated both by hand. This is where the Phase 4 ghost came from: `RemovePlayer`
    updated the map and forgot the room.
  - ~~**Mobs, two copies.**~~ `spaces.MobileRoomMap` held `mobileToRoom` (mob -> room), and
    each `Room` had its own `mobs`.
  - ~~**`moveMobile` only worked because errors were ignored.**~~ It called
    `src.MobileLeaves`/`dest.MobileEnters`, which moved the mob between room lists, and then
    `mobileRooms.Remove` + `Add`, which *also* touched the room lists: `Remove` removed the
    mob from `src` a second time (`ErrNotFound`, swallowed by a `// TODO error handling`)
    and `Add` added it to `dest` a second time (`ErrDuplicate`, swallowed). Correct result,
    by accident.
  - **Floor objects, one copy.** Only `Room.Inventory`. Nothing asks "which room is this
    knife in?", so there is nothing to desync. Unchanged, on purpose (step 5).
  
  *The decision: rooms keep their lists.* Almost every question the game asks starts from
  a room -- `look`, `say`, `Room.Send`/`Notify`, aggro's "first player in my room",
  `get knife`, corpse decay -- and the room's `ordered.List` answers it directly and in a
  stable order. Keeping only thing -> room in the world would make every `look` a scan of
  the whole world and lose that order. The reverse direction (player -> room, mob -> room)
  is an *index*: useful for `tell`, `stat`, saving and combat, but derived. Two copies is
  fine; two *writers* is the bug.

  *The plan: one owner, and nothing else can write.*

  1. ~~A type in `spaces` -- `Occupancy`, say -- holding both directions for players and mobs:
     `PlacePlayer(p, r)`, `MovePlayer(p, dest)`, `RemovePlayer(p)`, `RoomOfPlayer(p)`, and
     the same four for mobiles. It is the only code that touches a room's lists.~~ Done.
  2. ~~Unexport the room's writers (`AddPlayer`, `RemovePlayer`, `AddMobile`, `RemoveMobile`,
     `PlayerEnters`/`Leaves`, `MobileEnters`/`Leaves`), leaving `Room` with readers only:
     `Players()`, `Mobs()`, `FindPlayer`, `FindMobile`. Because `Occupancy` lives in
     `spaces` it can still call them; `world` can't. A half-done move now fails to compile
     instead of leaving a ghost.~~ Done.
  3. ~~Delete `roomToMobiles` and the `syncmap` dependency with it.~~ Done 2026-09-25.
  4. ~~`world.playerRoomMap` and `MobileRoomMap` fold into `Occupancy`; `World.movePlayer`,
     `moveMobile`, `addPlayerTo` and `RemovePlayer` become one call each plus whatever
     else they do (fights, `playerList`). Errors from the lists stop being swallowed --
     with one writer, a duplicate or a miss is a real bug worth logging.~~ Done, together
     with step 1.
  5. Leave objects alone. Where an object is forms a tree (floor, inventory, equipment,
     corpse contents) and nothing asks the reverse question yet. Add an index when a
     "locate object" spell or a script needs one, through the same owner.

  *Why this is the Lua groundwork.* "The janitor sweeps away a rusty knife" needs two
  things. Room-centred reads (`room:items()`, `room:players()`), which the room lists
  already give. And a single place where state changes: the script calls a world verb
  (`extract(obj)`, `move(obj, dest)`) rather than editing a list, and that verb is where
  the event goes out and where hooks fire -- `on_enter`, `on_leave`, `on_drop`. With moves
  spread across handlers, every hook would have to be added in several places, and one
  would be missed exactly the way `RemovePlayer` missed its room.
- **`spaces.Room` conflates definition and instance.** One struct holds both the static
  topology loaded from `content/` (`Id`, `Name`, `Description`, `Zone`, `directions`, `flags`)
  and the live contents that change every tick (`playerList`, `Inventory`, `mobs`). Because
  the loader is necessarily two-pass — exits are cyclic and cross-zone, so rooms are all
  constructed before `Content.connectRooms` wires them — `Room.Connect` has to be exported,
  and a handler can rewrite world topology at runtime by calling it. Hiding `Connect` would
  fix little: `Name`, `Description`, `Zone` and `Inventory` are exported fields anyway. The
  real fix is the split this codebase already applies everywhere else (see "Definition vs
  Instance" in CLAUDE.md): a `RoomDefinition` owned by the `Zone`, immutable once loaded and
  holding the exits, and a live `Room` pointing at it. Same refactor as the dual-bookkeeping
  item above, seen from the other side — do them together. Now unblocked.
- ~~**Nothing drives combat.**~~ Fixed: aggressive mobs start fights and they run to a
  death. See CLAUDE.md "Combat".
- ~~**The fight ledger leaks third-party attackers.**~~ Fixed twice over. `EndAllFightsWith`
  now clears the dead one from everyone's fights, and then retargets whoever that leaves
  being fought but not fighting -- the other half of the leak, where a mob whose target
  died stood still forever. `isBeingFought` is still a linear scan; fine at this size.
- **No scripting language.** Go is the primary language and stays that way: every
  feature and every mechanic -- combat, power, loot, abilities, threat, doors and locks
  -- is written in Go. Lua (gopher-lua) is for small, local behaviour that composes
  actions the engine already has: *a mob picks up an item it finds*, *a mob locks a door
  that's unlocked*, *a mob taunts the player it's fighting* ("Hah! You cannot defeat
  me!"), the Barrow-King saying something and summoning skeletons at half
  health, the hedge-witch answering `say heal`. A script decides *when*, never *how the
  math works*; if a script needs an action the engine doesn't have, that's a Go feature
  first.
  How it fits: scripts run on the world goroutine, one Lua state, no locks (gopher-lua
  isn't thread-safe and doesn't need to be); every hook call is time-limited, for the same
  reason `Send` never blocks; dice go through `w.roller` so scripted fights stay testable;
  Lua coroutines give `wait(2)` resumed by the heartbeat, which is why Lua over Starlark.
  Scripts live in `content/world/<zone>/scripts/`, named from mobs.json/rooms.json and
  resolved at startup. The real work is the hooks (entered room, died, health crossed a
  line, heard something, pulse) and a small action API over them.
  The first two examples need Go features that don't exist yet -- mobs have no inventory,
  and there are no doors or locks. Taunts need nothing new: `event.Said` already takes a
  speaker name and renders to the room. So taunts are the first case -- a fight-started
  and a fight-pulse hook, and one action, `say` -- and the King's half-health script is
  the second.
  **Taunts are done** (2026-09-28; spec in `docs/superpowers/specs/`, and CLAUDE.md
  "Scripts (Lua)"). Next is the King's half-health script, which brings `wait()` over
  coroutines and a "health crossed a line" hook.
- ~~**`Fight` snapshots `ZoneId`/`RoomId`** at the moment it starts, so a fight that somehow
  outlives its room notifies the wrong one.~~ Fixed 2026-09-26: a fight has no location;
  `DoViolence` asks `Occupancy` where the fighter is standing.
- **`RoleWeights` is hand-authored for everything that isn't armor.** ~~A builder writing
  `"roles": {"tank": 3}`~~ -- fixed for armor. A role declaring `"from_armor": true` in
  `roles.json` (Tank) is fed by `armor.json`'s armor-type-by-slot table, the same number
  that `Equipment.ArmorClass` adds to AC, so armor is tuned once and nobody types a weight.
  It remains true for weapons and everything else: a knife's `"roles": {"striker": 2}` is
  still a number somebody picked, and when weapons grow real damage stats it will be the
  same inconsistency in the same shape. Derive it from the weapon then; don't add a third
  axis.
- **`Catalog.RoleFor` is an argmax, so it has cliffs.** Plate plus a censer is Tank or
  Healer depending on a tiebreak, with nothing in between, and one point of weight flips
  it. This is fine -- genuinely fine, not tolerated -- while a role is a label a player
  reads off `stat`, `role` and `who`. It stops being fine the instant anything mechanical
  branches on the result, because a cliff in a display string is a cosmetic surprise and a
  cliff in a damage formula is a balance bug. **The fix is not a smoother function; it is
  not branching on it.** Combat that wants tankiness should read the armor, not the label.
- ~~**`server.handleLogin`** logs the error from `player.FromRecord` and then falls through
  and uses the player anyway.~~ Already fixed: it returns the error.
- **`world/settings.go`** is a single `VERBOSE_LOGGING` const, and logging is split between
  zerolog and stdlib `log` depending on file age. Worth one consolidating pass eventually.

---

## Verification

Per phase:
- **0–3:** `make test` green after each. Phase 3 additionally: server starts and idles
  without pegging a core.
- **4: DONE.** All of the below was exercised by hand, plus two automated suites that
  need no socket: `telnet/render_test.go` drives command strings through `NewTestWorld`
  and asserts on rendered text (parser, dispatch and renderer in one pass), and
  `telnet/protocol_test.go` covers the IAC filter including subnegotiation payloads that
  contain `0x00` and doubled `0xFF`.
- **4 (original plan):** manual `telnet localhost 4000` — create a player, `look`, move between rooms in
  `content/world/wrathrock`, `get`/`drop`/`inventory`/`wear`, `kill` a mob from
  `content/world/sample`, `who`, `quit`, reconnect. Two simultaneous connections to confirm
  `say`/`tell` notifications reach the other session. Watch that mob wandering (10s pulse)
  and zone reset (3min lifetime in `zone_manifest.json`) still fire while a client is idle.
- **5: DONE.** `make check` (fmt-check + vet + test) is green — vet for the first time ever.
  Verified live as well, with two simultaneous telnet sessions, because the merged
  audience-aware events are exactly the thing a single-connection test cannot check: the
  actor must see "Dropped." while the bystander sees "testdood drops knife." Both
  `telnet/render_test.go` cases for that passed with their expected text **unchanged**
  through the whole conversion, which is the evidence no player-visible string moved.
- **6: DONE.** `make check` green, plus `world/h_role_test.go` and `world/h_remove_test.go`
  for the derivation and the slot bookkeeping, and cases in `telnet/render_test.go` for
  `role`, `stat`, `who` and `remove`. Verified live, which is where the missing `remove`
  turned up: create a character, pick a lineage from the menu, `load` the sample zone's
  chain shirt / tower shield / censer / knife, and watch `role` and `who` follow the gear
  as it goes on and comes off.
- **7:** connect with tintin++ rather than raw telnet; verify negotiation, wrapping, color.

Regression net worth adding early (cheap, and it makes every later phase safer): a table test
that feeds command strings through the Phase 4 parser into a `newTestWorld()` and asserts on
rendered output text. That covers parser, dispatch, and renderer in one pass without a socket,
and it survives the protobuf decision in Phase 5 unchanged.
