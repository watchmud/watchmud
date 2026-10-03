# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this
repository.

## What this is

A text MUD server in Go that speaks **telnet**. `make run`, then `telnet localhost 4000`,
and you can create a character and play.

The vocabulary is two packages of plain Go structs: **`command/`** is what comes in,
**`event/`** is what goes out. `telnet/parse.go` turns a typed line into a `command.Command`;
handlers answer with events; `telnet/render.go` turns those into text. Nothing is serialized
anywhere, because nothing needs to be.

Classes are **gone** (Phase 6, September 2026) and lineage is cosmetic: see "Lineage and
Role" below. Protobuf is **gone** (Phase 5, September 2026), and with it `github.com/trasa/watchmud-message`,
gRPC, and the `GameMessage` oneof wrapper. The old gRPC transport (`rpc/`), the static web
page (`web/`), the Postgres layer (`db/`), and the `client.Client` abstraction were deleted
earlier. The separate console client repo `trasa/watchmud-client` is no longer used by
anything.

## Commands

```
make build            # -> bin/watchmud, bin/watchmud-bot, bin/watchmud-bots
make test             # go test ./...
make run              # build + run with the default ./app.local.yaml
make fmt-check        # gofmt -l .  (really does fail now)
make vet              # green since Phase 5
make generate         # regenerate *_string.go after editing a stringer enum
make db-up            # start the local mongo (docker compose, host port 27018)
make test-db          # the mongostore tests that need a real mongo
go test ./bot         # the smoke bot, including the real world in-process
make docker-build     # build the deploy image (deploy/README.md for the rest)
go test ./world -run TestLook_successful          # single test
go test ./player -run TestPlayerTestSuite/TestX   # testify suite: Suite/Method
```

`behavior.Behavior` and `object.Category` carry `//go:generate go tool stringer` and need
`make generate` after adding values; stringer is a `tool` directive in go.mod, so there is
nothing to install.

### Baseline (as of 2026-09-12, master)

**`make check` is green** -- fmt-check, vet and test all pass. That is new as of Phase 5 and
it is the gate: if any of the three is red, you broke something.

The long-standing `copylocks` noise is gone because its cause is gone. It was never a style
problem: the codebase passed protobuf structs by value, and those embed a
`protoimpl.MessageState` containing a zero-sized `sync.Mutex` put there precisely so vet
would complain. `command`/`event` structs are ordinary values with nothing to copy.

## Architecture

### The tick loop is the whole concurrency model

`server.GameServer.Run` (server/gameserver.go) is a single goroutine that selects on three
things: context cancellation, the `incomingBuffer` channel of client messages (buffered 64,
dispatched immediately), and a `mudtime.PulseInterval` ticker (1s) that runs `heartbeat`.
`heartbeat` does pulse work only -- zone resets (`PulseZone`), mobile wandering
(`PulseMobile`), combat rounds (`PulseViolence`).

Commands are dispatched the moment they arrive rather than on the next tick, which is what
makes typing feel responsive; but they are still handled one at a time on this goroutine,
never concurrently with a pulse. So `World` and everything it owns is effectively
single-threaded and carries no locks. **Don't add goroutines that touch world state** --
push work onto `incomingBuffer` or into the heartbeat instead. Connection goroutines are
allowed to exist only because all they do is send on that channel.

### Message routing

Two-level dispatch, both of them type switches. `GameServer.dispatch` handles the pre-world
cases itself (`command.Login`, `command.CreatePlayer`), since those need the store and
player construction. Everything else falls through to `World.HandleIncomingMessage` in
world/handlers.go, whose switch *is* the whole dispatch table -- no string keys, no
reflection -- and which hands each handler its command already typed.

Adding a command:

1. `command/commands.go`: the struct plus its one-line `Verb() string`.
2. `event/events.go`: whatever it answers with, if it needs a new event.
3. `telnet/parse.go`: the verb and its aliases.
4. `world/h_<name>.go`: `func (w *World) handle<Name>(msg *gameserver.HandlerParameter, cmd command.<Name>)`.
5. `world/handlers.go`: a `case command.<Name>:` in the switch. **A type switch has no
   exhaustiveness check** -- forgetting this compiles fine and fails at runtime into the
   default arm. `world/world_unknownMessage_test.go` covers that arm; your own case needs a
   test in `telnet/render_test.go`.
6. `telnet/render.go`: a case for the event.
7. `telnet/help.go`: an entry, if players should know about it (builder commands
   shouldn't). `help_test.go` fails if help offers a verb `parse.go` doesn't know.

Handlers reply via `msg.Player.Send(...)`, `room.Send(...)` or `room.Notify(...)`, and fail
via `msg.Fail(event.SomeCode)`. They never return errors, and **`Send` has no error return**
(see Conventions).

Persistence is on a timer, not per command. Every `playerSave` (`content/rules/mudtime.json`)
the heartbeat calls `World.QueuePlayerRecords`, which hands a record of everyone in the
world to the store; `Run` does the same once more on shutdown. Handlers mutate the player
and never save -- whatever changed a player, a command or a fight or somebody else's
command, is in the next one. Logout saves immediately, as does death; a crash loses at
most one interval.

**Save through `w.record(p)`, never `p.Record()` directly.** The player doesn't know where it
is standing -- location lives in `spaces.Occupancy` -- so `w.record` is what fills in
`LastZoneId`/`LastRoomId`, and a bare `p.Record()` saves a player who is nowhere. Logout
takes the record *before* `RemovePlayer` for that reason.
On login, `World.ReturnPlayer` puts them back in that room, falling back to the start
room when the record names none (a new or pre-location character) or a room content no
longer has. `AddPlayer` is the start-room path, for new characters and tests. After the
login event, `GameServer` calls `World.Arrive`, which tells the room (`event.EnteredGame` --
not `LoggedIn`, which every connection would consume as the end of its own login) and shows
the player the room. It is kept out of `AddPlayer`/`ReturnPlayer` so the description lands
after the telnet login conversation has finished.

`player.Store` has two implementations: `memstore` (a map, used by every test) and
`mongostore` (one document per character in the `players` collection, chosen when
`mongo.uri` is set in the config). `mongostore` keeps its own `playerDoc` with the bson
tags and string-ified uuids rather than tagging `player.Record`, so `player/` stays
unaware of the database -- and because `uuid.UUID` is a `[16]byte` that bson would
otherwise write as sixteen numbers. The mongo tests skip unless `WATCHMUD_TEST_MONGO_URI`
is set, so `make check` still passes with no docker.

In the server, either one is wrapped in **`writebehind.Store`**, so `Save` only queues and
returns: one background goroutine writes, keeping the newest unwritten record per player,
skipping records identical to the last one written, and using `SaveAll` (one mongo
`BulkWrite`) when the inner store has it. `Load` answers from that queue first, which is
what makes quit-and-log-straight-back-in safe. Two consequences:

- **Nothing on the world goroutine hears a save fail.** In particular mongo's unique name
  index can no longer refuse a duplicate at creation, so `handleCreatePlayer` checks
  `store.Load` itself (`event.NameTaken`). Creation spans two dispatches -- bcrypt runs on
  a goroutine in between -- so `handleCreateHashed` checks again when the hash comes back;
  each check alone is race-free because dispatch is one command at a time. `handleLogin`
  likewise refuses a character already in the world (`event.AlreadyPlaying`), and checks
  again in `handleLoginChecked` -- two sessions of one character save over each other.

**A name has one stored form.** `player.CanonicalName` turns whatever was typed into it
("bOB" is "Bob") or refuses it, at the top of `handleLogin` and `handleCreatePlayer`;
everything after that, the store included, can compare exactly. Where a player types a
name in play (`tell bob`), `player.List` keys on `player.NameKey`, the lowercase. Don't
add a case-insensitive lookup to a store: canonicalize at the door instead.
- **Shutdown order matters.** `main` defers `saver.Close` after `closeStore`, so it runs
  first and flushes before mongo disconnects. It only runs on a signal `NotifyContext`
  listens for: SIGINT and SIGTERM. Anything else exits without the last save.

### The transport seam

`gameserver` is the seam package and owns all three sides of it:

- `Instance` -- what a transport calls into the game (`Receive`, `Logout`).
- `Conn` -- what the game calls back out to (`player.Sender` + `Player`/`SetPlayer`/`Close`).
  It exists *only* because a connection has no `*player.Player` until it logs in; once it
  does, everything talks through the player instead.
- `HandlerParameter` -- what a handler is handed. `NewHandlerParameter` snapshots
  `c.Player()` at construction.

`gameserver.TestConn` is the test double. Nothing in `world/` knows what transport is talking.

### telnet/

`telnet.Listen(ctx, addr, gs)` accepts connections and gives each one a `conn` with two
goroutines:

- **`readPump`** -- `bufio.Scanner` over `iacFilter` over the socket. Runs `login()`, then
  `commandLoop()`.
- **`writePump`** -- drains `sendQueue`, renders, writes. It owns the socket's lifetime:
  `Close()` closes `quit`, and `writePump` drains what's left (so the goodbye lands) before
  calling `netConn.Close()`, which is what unblocks a parked `readPump`.

Two rules that look inconsistent and are not:

- **`Send` must never block.** It runs on the world goroutine, so a slow client would freeze
  the entire MUD. It does a non-blocking send and hangs up on overflow.
- **`emit` is allowed to block.** It runs on the connection's own goroutine, and blocking on
  a full `incomingBuffer` is correct backpressure -- the flooding client waits, no command
  is dropped.

**`tls.go`** is the second port: the same `conn` over `tls.Server`, handshaken on its
own goroutine before the pumps start. `certificate` re-reads the PEM files when their
mtimes move (checked each handshake), so a Let's Encrypt renewal needs no restart --
and a restart is an outage for everyone connected. `Listen` runs both listeners with
one `addressLimit`, and returns the first one's error, which `main` treats as fatal.

**`protocol.go`** is an `io.Reader` that strips `IAC` sequences, including subnegotiation
payloads that legitimately contain `0x00` and doubled `0xFF`; it also hands each
WILL/WONT/DO/DONT the client sends to `conn.negotiated`, and each subnegotiation (capped at
`maxSubneg`) to `conn.subnegotiated`.

**Wrapping is to the client's window, and only if it says** (`telnet/wrap.go`). After the
banner the server sends `DO NAWS`; a client that answers with its size gets every line
wider than that rewrapped at spaces in `conn.frame`, after color is decided -- color takes
no columns -- with a line's leading spaces carried onto its continuations. A line that fits
is untouched, so padded tables keep their columns. No NAWS means no wrapping, which is
what the bots get. The width reaches `writePump` through the queue (`windowSize`), like
everything else readPump learns.

**Every prompt is marked**, the in-game one and each login question (`question`, so
`write` can tell): `IAC GA` after it, or `IAC EOR` once the client answers the
`WILL EOR` sent after the banner with `DO`. A prompt has no newline, so without the mark a
MUD client can only guess from a pause whether a partial line is a prompt; tintin's
`#action {^Password:}` depended on that guess. A prompt `frame` doesn't print gets no mark --
a bare GA would announce a prompt that never came.

**Color is the renderer's, and the connection's to take away.** `render` always paints
(`telnet/ansi.go`: the palette is named by what a thing *is* -- `colorRoomName`,
`colorMob` -- so restyling is one file), and `conn.frame` strips it with `plain` unless the
player wants it. The connection learns that from `event.Color`, which `World.Arrive` sends
silently ahead of the first room and `color [on|off]` sends with `Changed` set; until then
it's plain, so the login conversation never has any. The choice is `NoColor` on the record,
inverted so old records and new characters are both on. Tests about *wording* compare
`plain(render(...))`; `ansi_test.go` is where the colors are pinned. The bot drops color as
a terminal would (`bot/telnet.go`), so a palette change never breaks a bot pattern.

**Login is straight-line code, not a state machine.** `login()` blocks on an `authResult`
channel that `Send` signals when one of `event.LoggedIn` / `PlayerCreated` / `LoginFailed` /
`CreateFailed` passes through. Those four are why login is the one place a bool would have
been tempting: the consumer is a connection state machine, not the renderer, so they stayed
four distinct types instead of collapsing into `event.Failed`. The connection's mutex guards
`player` and nothing else.

**The prompt is `event.Prompt`, and the server sends it indiscriminately**: to
every player in the world after every dispatched command and every heartbeat
(`GameServer.prompt`). `conn.frame` decides whether to print it -- only when something was
written since the last one, never before login (no player yet), and anything arriving while
a prompt sits unanswered is pushed onto its own line. When `readPump` reads a line it queues
`inputReceived`, so `writePump` knows the prompt was answered; it goes through the queue
rather than a field so it lands ahead of the world's reply. The two things the world never
hears about -- a bare Enter and a line that doesn't parse -- queue `reprompt`, which repeats
the last `event.Prompt` the server sent. The prompt shows health and mana
(`<97/100hp 80/100m> `), which are world state, so the connection never builds one itself: only the server fills one in.
`atPrompt` and `lastPrompt` belong to `writePump` alone.

**`parse.go` and `render.go` are the two chokepoints**, one each way, and neither holds game
logic. `parse.go` maps a verb (and its aliases) to a `command.Command`; it never parses
target strings -- `world.parseTarget` owns the `all` / `all.knife` / `2.knife` / `20 coins`
grammar, and duplicating it in a transport is how the two drift apart. An empty target is
not a parse error either; the handler answers `NO_TARGET`.

`render.go` is one type switch, one case per event. Events that bystanders also see arrive
here once per player and compare their `Actor` field to `self` -- that is why there is no
separate notification type for drop, get, say, tell or shout.

`resultcode.go` is separate on purpose: it maps `event.ResultCode` to player-facing text and
is keyed on the code, not on any type. It takes a verb as well, because `TARGET_NOT_FOUND`
means "you don't see that here" to `get` and "you aren't carrying that" to `drop`.

Renderers emit plain `\n`; `conn.write` does the single CRLF translation. Keep it that way --
expected strings in tests stay readable, and it is impossible to forget.

### bot/

**The bot is a player, not a peer.** `bot.Client` is plain TCP: `Send` a line,
`Expect` a regexp against what arrived since the last match. It imports nothing of the
server -- no `event/`, no `telnet/` -- so it reads the text a player reads, and a
rendering change that breaks it is one a player would have noticed too. Fix the bot's
pattern, never the renderer to suit the bot.

`bot.Smoke` is the one scenario: recall, the walk to the millpond, the geese, `get all
from corpse`, `drop all.feather` (never `drop all` -- the character keeps its starting
kit), then `recall` again before `quit` -- it logs back in where it quit, and the geese
will have respawned by the next deploy. It never answers the creation question, because
names are permanent.
`cmd/watchmud-bot` runs it, and `deploy.sh` runs that after every restart (deploy/README.md,
"Smoke test"). `bot/smoke_test.go` runs it against the real `content/` in-process, at
`GameServer.SetTickInterval(10ms)`, so a content change that breaks the walk fails `make
check`. If you move the geese or rename a room on the route, update `route` in
`bot/smoke.go`.

`bot.Adventurer` is the inhabitant: a state machine (goHome, town, travel, hunt,
donate; fight and rest are procedures) driven by `Client.ReadChunk`, one prompt's
worth of output at a time. Its manners are constants, not config: it never fights
in a room with a real player (siblings are known by name) and leaves that ground for
10 minutes -- on foot (`ground.homeward`, walked from every patrol room by
`TestGrounds_walk`), never by recall, so a player sees bots walk off one at a time instead
of a crowd vanishing. A bot with nowhere to hunt waits in Market Square (`waitIn`), never
Temple Square: that's where every recall, death and new character lands; `consider` gates every kill; it answers each person's tell once per 10
minutes with the same honest line. Hunting grounds (`bot/grounds.go`) are hand-written
tables walked against the real content by `TestGrounds_walk`; never add the
hedge-witch as prey. There are two, the west and east fields: a bot is power 1 for good
(it never wears loot) and fights nothing above power 2, which is only the farms. It picks
at random among the grounds its power suits and nobody is using, so bots spread out and a
player on one sends them to the other. `cmd/watchmud-bots` runs up to 5 (one address, the server's
cap), reconnecting forever.

### Content loading (loader -> world)

`loader.LoadContent(os.DirFS(contentPath))` reads `content/` into an immutable
`loader.Content{Zones, Settings, Catalog}`; `world.New(content, store)` then builds the live
world from it. The split matters: `Content` is static definitions, `World` holds instances
and who-is-where.

Layout under `content/`: `world/settings.json` (the start, void, donation and
player-death rooms, each a `{ "zone_id", "room_id" }` pair -- `loader.RoomRef`; every one
but donation is resolved when the world is built, so a bad reference fails startup --
and `welcome`, the line a new character is told after being shown the start room:
`World.Welcome`, called from creation only),
`world/zone_manifest.json` (which zones load, reset mode, lifetime), then per-zone
`world/<zone>/{rooms,objects,mobs,instructions}.json` -- only rooms.json is required.
`rules/{species,roles,armor,starting_gear}.json` feed `rules.Catalog`.

`rules/starting_gear.json` is what a new character is created holding: object definitions
named by zone, each optionally `"equip": true`. It carries no slot -- `object.Definition`
already names the one slot the item goes in. `loader.Content.checkStartingGear` runs last,
after the zones, and fails startup on an unknown zone or object, an `equip` on something
unwearable, or two items claiming one slot; `player.GiveStartingGear` then hands the kit
out in `handleCreatePlayer`, before the first `store.Save`, so it is on the record from the
start. It is creation-only -- a returning player's gear comes back from their record, and
calling it on a login would hand out a second copy of everything.

Room loading is deliberately two-pass (all rooms constructed, then exits connected) because
exits cross zones. Zone iteration is always over `slices.Sorted(maps.Keys(...))` so load
order and error messages are reproducible -- keep that when adding loaders, and prefer
deterministic iteration generally: every container a player can see the contents of had to
be made stable, because a room's contents and a player's inventory were shuffling on every
`look` -- see `ordered.List` below.

`instructions.json` entries become `spaces.ZoneCommand`s (`CreateObject`, `CreateMobile`)
that `Zone.Reset` replays -- at startup and again on every zone reset -- which is how mobs
and loot repopulate.

### command/ and event/

Both are **leaf packages**: they import `direction`, `slot` (both top-level packages in this
repo since Phase 5 -- they used to live in the message module) and the standard library, and
nothing else. No `*player.Player`, no `*spaces.Room`, no `*object.Instance` in a field --
only strings, ints and typed enums. `spaces` imports `event` (for `Room.DescriptionExcept`),
so anything richer is a cycle waiting to happen.

Two rules that carry most of the weight:

- **A success event carries its payload and nothing else.** There is no `Success bool` and
  no `ResultCode`; failure is `event.Failed{Verb, Code}`, and `msg.Fail(code)` fills the verb
  in from the command. That is one renderer case for every failure in the game.
- **One event per thing that happened**, with an `Actor` where bystanders see it too. The
  renderer picks the wording. `event.RoomDescription` covers look, move and recall alike.

`command.Command` is `interface{ Verb() string }`. The verb is the canonical command name,
not what the player typed (`l` and `look` are both `command.Look`), and it exists so a
failing handler never has to repeat its own name.

### Lineage and Role

**A lineage is cosmetic and a role is not stored.** Both halves of that are load-bearing.

`rules.Lineage` (Hill Dwarf, Wood Elf, ...) grants nothing: no bonuses, no resistances, no
anything. `rules.Species` survives only to group lineages for the creation menu. Character
creation picks a lineage and picks nothing else. This is why `player.FromRecord` no longer
refuses a login over a lineage the catalog doesn't recognize -- nothing depends on it, so
it logs and falls back to `Catalog.DefaultLineage()`. A future "change your lineage" potion
needs no rebalancing because there is nothing to rebalance.

**There are no ability scores.** `rules.Abilities` and its six str/dex/con/int/wis/cha
numbers were deleted in Phase 6 rather than left lying around. Once a lineage granted no
bonuses and there was no class to prefer one score over another, every character had the
same array and nothing but `stat` ever read it -- which makes it a tempting place to route
a future mechanic through, and routing mechanics through a hidden number is the opposite of
what this design is for. If real combat stats land later, derive them from equipment.
`player.Record` has no `Abilities` field; don't add one back without deciding what reads
it.

`rules.Role` (Tank, Healer, Striker -- `content/rules/roles.json`) replaces the class, and
**the gear is the truth**. `object.Equipment.RoleWeights()` sums what everything equipped
argues for and `Catalog.RoleFor` picks the highest total, ties going to whichever role
`roles.json` declared first. Armor in every slot makes you a Tank; take it off and hold a
censer and you are a Healer before the next prompt.

A piece of gear argues for a role in one of two ways, and `Equipment.RoleContributions()`
is the single list of both:

- **Armor derives it.** A role with `"from_armor": true` in `roles.json` -- Tank -- is fed
  by `content/rules/armor.json`, a table of armor type (cloth/leather/plate) by slot.
  `Catalog.ArmorWeight` looks the piece up by its `"armor_type"` and the slot it's worn in.
  The same number is the AC the piece adds (`Equipment.ArmorClass`), on purpose: armor that
  protects you more is armor that argues harder that you're the one standing in front, so
  there is no second number to keep consistent with the first.
- **Everything else declares it.** `object.Definition.RoleWeights`, a `map[roleId]int` from
  the `"roles"` key in `objects.json`, still exists because a knife or a censer has no armor
  type to derive anything from. Armor may use both; a blessed helm can argue for Healer on
  top of what its plate is worth.

`Equipment` holds the `*rules.Catalog` so `ArmorClass()` and `RoleWeights()` can stay
zero-argument -- `combat.Combatant` requires the former and every handler expects the
latter. That is not a role being stored: nothing is cached, and every answer is recomputed
from what is in the slots at that instant.

So: there is no `Role` field on `player.Player`, no `RoleId` in `player.Record`, and no
command that sets one. Storing it would let it disagree with the equipment. Anything that
wants a role name calls `World.roleName(p.RoleWeights())` (`world/world.go`), which needs
`w.content.Catalog`.

**Nothing mechanical may branch on a role, and that is a rule rather than a "not yet".**
`RoleFor` is an argmax, so it has cliffs: plate plus a censer is Tank or Healer on a
tiebreak with nothing in between, and a point of weight either way flips the answer. That
is harmless while a role is a label a player reads. It becomes a balance bug the moment
combat asks "am I a Tank?" and gets a different answer than the player expected. If you
want tanky behaviour, read the armor, not the label.

The hand-authored-second-axis problem ROADMAP listed under "Known problems" is fixed for
armor, which now derives its weight from its armor type. It remains for everything else:
a knife's `"roles": {"striker": 2}` is still a number a builder picks, and if weapons grow
real damage stats it will be the same inconsistency again, in the same shape. Derive it
from the weapon rather than keeping two numbers in step.

### Abilities

**What you can cast comes from what you have on**, the same way role and power do. An
object grants abilities with `"abilities": ["heal"]` in objects.json
(`object.Definition.Abilities`); `Equipment.Abilities()` answers what the equipped,
**unbroken** gear grants right now, as a `map[abilityId]Grant` -- where two items grant
the same one, the higher-power item wins (a tie goes to the first in slot order), and
that item's power is what the ability is cast at. Nothing on `player.Player` or
`player.Record` lists what a player can do; a censer in the pack does nothing until it's
held. Not to be confused with the ability *scores* Phase 6 deleted (`rules.Abilities`,
above): a `rules.Ability` is something gear lets you do, not a number about you.

Two halves, each checked at startup so content can't name an ability that does nothing:

- **The numbers are content**: `content/rules/abilities.json`, loaded by
  `Catalog.SetAbilities` -- id, name, mana cost, cooldown, a `target` kind (`self`,
  `friend`, `foe`, `none`), and each ability's own parameters (heal's `amount`: `base +
  per_power * power`). A bad target, a negative cost, or an object granting an id the
  catalog lacks fails the load.
- **What it does is Go**: `effects` in `world/abilities.go`, an `effect` per ability id.
  `world.New` fails if a catalog ability has no effect (`checkEffects`). An effect can't
  refuse -- by the time it runs, the cast is paid for.

`cast <ability> [target]` (`world/h_cast.go`) refuses in this order, spending nothing:
no ability given, not in the catalog (`UNKNOWN_ABILITY`), not granted by anything
equipped (`NOT_GRANTED`), on cooldown (`NOT_READY`), target not resolvable
(`TARGET_NOT_FOUND`), not enough mana (`NOT_ENOUGH_MANA`). **A cast that passes always
spends** its mana and starts its cooldown, even a heal on someone who isn't hurt -- wasting
it is the player's mistake to make, and `event.Healed` carries an `Amount` of 0 so the
renderer can say so. `castTarget` resolves the target once, by kind, so no effect parses
a target string: `friend` is the caster when empty, otherwise a player **in the caster's
room**. `foe` and `none` exist in the catalog but not yet in `castTarget`; the first
ability that needs one adds it.

**Mana** sits beside health: `rules.MaxMana` (100) for everyone, flat, so gear decides
*what* you cast, not how much. `CurMana` on the record is a `*int` for the durability
reason -- a record from before mana reads as full. It comes back on the regen pulse
**fighting or not**, unlike health (`rules.ManaRegenPercent`): a healer with nothing left
to cast is a spectator. **Cooldowns are never saved** (`Player.readyAt`, in memory): a quit
resets them, and logging back in takes longer than any cooldown. They read the clock
through `World.now`, which tests replace instead of sleeping; nothing else uses it yet.

The prompt shows mana after health (`<97/100hp 80/100m> `) when `event.Prompt.MaxMana`
is set -- always, for a real player; test fixtures that leave it zero render as they did.
`abilities` lists what the gear grants, in catalog order, with the item, its power and
the seconds until ready (rounded up, so it never promises a cast too early).

Heal is allowed mid-fight -- that's what it's for -- and neither starts nor joins one.
Nothing about abilities reads a role; the Healer label and the censer's heal are two
separate consequences of the same item.

**Adding an ability:** an entry in abilities.json (with any new parameter as a field on
`rules.Ability`), an effect in `effects`, a case in `castTarget` if its target kind is
new, its own event (`Healed` is heal's; there is no generic `Cast` event) and a render
case for it, and `"abilities"` on the objects that grant it. Then a test in
`world/h_cast_test.go` and a `telnet/render_test.go` case.

### Durability

**Durability is the first mutable thing an `object.Instance` has ever had**, and it is
per-instance for the obvious reason: the chain shirt you have been fighting in is not in
the same state as the one on the shelf. `Definition.MaxDurability` is assigned by the
loader (like `RoleWeights`) from `content/rules/durability.json` -- by armor type, with a
`default` for everything else and an optional per-object `"durability"` override.

`rules.Indestructible` (zero) means gear that never wears out, and that is the default
everywhere: no durability.json is durability switched off, not a world of gear born
broken. The same rule governs the save record, where `InventoryRecord.Durability` is a
`*int` so a record written before durability existed reads as "as new" rather than as
zero. Both of those are the same trap as the mob `ac` one above; both are pointers for
the same reason.

**Broken is not destroyed.** A broken piece stays worn and stays in inventory, and stops
counting: `Equipment.ArmorClass` and `RoleContributions` both skip it, so a breastplate
giving out costs the player AC and can flip their role. That is what `repair` acts on
(`world/h_repair.go`): in a room flagged `smithy` -- Wrathrock's, west of Market Square --
`repair <item>` or `repair all` puts what's carried or worn back to `MaxDurability`,
broken included, for coins: `rules.Economy.RepairCost`, half the piece's price from broken
to new and proportionally less for less wear. Each piece is paid for as it's mended, so
`repair all` short of coins mends what it can in order and answers the rest with
`event.TooExpensive`.

Two things drive wear, both from `world/durability.go` (not `combat/`: combat works in
`Attacker`/`Defender` terms, knows nothing about equipment, and mobs have none):

- **A landed blow**, from `DoViolence`. One point off one piece of the defender's armor
  (`Equipment.DamageableArmor`, picked with `w.roller` -- a full suit shouldn't wear five
  times faster than a shirt) and one point off the attacker's wielded weapon. A miss costs
  nothing.
- **Dying**, from `combatantDied`. `DurabilityTable.OnDeathPercent` of what *every* worn
  piece started at (`Equipment.DamageableGear`, weapon included), at least one point each.
  A percentage rather than flat points so a death costs the same fraction of cheap and
  expensive kit; the player gets one `event.GearDamaged` summarising it, since the death
  where nothing breaks would otherwise be invisible. Anything that breaks announces itself
  with the same `event.Broke` a fight uses.

Message order matters and is pinned by tests: the blow, then the death, then what the
death cost, then the room the player wakes up in.

### Loot and corpses

A mob's `"loot"` in mobs.json (`mobile.Definition.Loot`) is resolved at startup -- a bare
object id is the mob's own zone, `"zone/id"` any other, and anything that doesn't resolve
fails the load. On death `World.rollLoot` rolls each entry on its own, a d100 (`IntN(100)`)
against its chance, and a drop is made at **the mob's power**, plus `rules.LootPowerBump`
from a second d100. Never the killer's power: out-levelling a boss makes his drops not
worth having, on purpose.

The drops go into the corpse, which is the only container so far. A container is an
`object.Instance` with non-nil `Contents` (an `object.List`, like the floor and inventory);
nil means "not a container", so check that rather than the category. Corpses are
`NoTake`, answer to `corpse`, and have a `DecaysAt`: `World.DecayFloors` runs on the
mobile pulse and removes them, contents and all, after `rules.CorpseDecay`. The zero
`DecaysAt` means never. Anything a character drops gets a `DecaysAt` too
(`rules.DroppedDecay`, 30 minutes), and `get` clears it -- so the donation room the
bots fill turns over. Zone resets and wizard `load`s never go through `drop`. `get <item> from <container>` and `look in <container>` live in
`world/containers.go`, and only search the room's floor.

### Economy

**Prices are derived, not written on objects** (`rules.Economy`, from
`content/rules/economy.json`): what a power-1 piece of a category is worth, times the
instance's power. Repair (`RepairCost`, half the price from broken to new), what a store
pays (`SellPercent` of the price, scaled by wear, so broken is worth nothing) and what a mob
carries (`CoinsPerPower`) all hang off that one table, through `world/economy.go`, so they
can't disagree. Absent is switched off, like durability. The loader refuses a
`sell_percent` of 100 or more: a store that pays what it charges mints coins. Selling pays
at most 40% and a full repair costs 50%, so no loop of buy, repair and sell makes money --
`TestSell_noProfitInBuyingAndSellingBack` pins one; keep it that way when tuning.

Coins are a number, not objects: `Player.coins` (on the record), and `Instance.Coins` on a
container, which only a corpse is. `get all from corpse` takes them, `get [n] coins from
corpse` just them; they go with the corpse when it crumbles, and a player keeps theirs on
death. A shop is a zone's `shops.json` (room, and objects at a power, named the way loot
names them) loaded into `spaces.Zone.Shops`; it sells new instances of its stock without
end, buys anything worth a coin, and never resells what it bought.

### Player death

A player leaves **no corpse**. `combatantDied` ends their fights (via `becomeCorpse`),
tells the room (`event.Died` with `IsPlayer`, which renders as "You are dead!" to the one
who died), takes the durability toll above, and then `playerRevives`: `Player.Revive()`
sets health to 1 and they are moved to `World.DeathRoom` -- the `player-death` entry in
`settings.json` -- and shown it. Health comes back from there on the `regen` pulse
(`World.Regenerate`, `world/regen.go`: 5% of max every 5s to anyone not fighting --
LEVELS.md). Mana comes back on the same pulse, fighting or not: a healer with nothing
left to cast is a spectator. `playerRevives` saves the player itself rather than waiting for the timed save.

**Slot order is `rules.CompareSlots`, not alphabetical.** `EquipmentSlot` is a string now,
so sorting the slots themselves puts `about_body` first and `wield` second to last.
`equipmentSlots` in `rules/equipmentslot.go` is declared weapon-first then head-to-toe, and
`EquipmentSlot.Rank()` is its index. `Equipment.All()` is an `iter.Seq2` that yields
occupied slots in that order, so the `equipment` listing, the `role` sources and the save
record all agree and none of them repeats the nil check.

Two consequences worth knowing before you touch equipment code:

- **`remove` is not a nicety.** Without it a character can add gear but never swap it, which
  makes a gear-driven role a one-way door. `handleRemove` empties the slot and leaves the
  item in inventory, since wearing something never took it out.
- **An empty slot deletes its key.** `Slots.Clear` deletes rather than storing a nil,
  because `IsSlotInUse` and the equipment listing both test for presence. A nil left in the
  map reads as "occupied" and would make the slot unusable forever.

Adding a role is a content edit (`roles.json` plus `"roles"` on some objects), not a code
change. An object naming a role the catalog doesn't define is a hard load failure.

### Scripts (Lua)

**Go is the engine; a script decides *when*.** A script composes actions the engine
already has -- today, one: `me:say` -- and never does the math. An action a script needs
that the engine lacks is a Go feature first.

A mob names a script in mobs.json, `"script": "barrow_king"` (bare is its zone,
`"zone/name"` any other), and the file is `world/<zone>/scripts/<name>.lua`. The loader
compiles every named script (`script.Compile`) into `loader.Content.Scripts`; a missing
file, a syntax error, a top level that errors or runs too long, or an unknown `on_` hook
fails startup. `world.New` loads them into one `script.Runtime`: one gopher-lua state, on
the world goroutine, no locks.

Hooks, both optional: `on_fight_start(me, foe)` (fired by `World.startFight` -- which
`kill` and aggro both use -- for each mob that wasn't already fighting) and
`on_fight_pulse(me, foe)` (from `DoViolence`, after the blow, never over a body). `me` is
copies (`name`, `health`, `max_health`), `me.memory` (a table per mob instance, dropped
with the mob in `World.RemoveMobile`), and `me:say`, bound to that one call. `foe` is
`name` and `is_player`. `chance(pct)` and `pick(list)` roll through `w.roller`, only
inside a hook; `math.random` is gone so tests can load the dice.

**A bad script mustn't hurt the server.** Every call runs under `script.CallTimeout`
(10ms), a capped call stack and registry, gopher-lua's protected call and a `recover`.
A failure is logged and the mob carries on; after `script.MaxFailures` (3) the program is
switched off until restart. Only base, string, table and math are open, minus anything
that loads code or prints. Three limits exist because of what the deadline can't see:

- **The deadline is checked between Lua instructions, not inside Go.** A builtin that
  does unbounded work in one call runs to the end with the world waiting. So the string
  library has no pattern functions (`find`, `match`, `gmatch`, `gsub` -- a bad pattern
  backtracks for minutes), `rep` refuses a result over 4KB, and `format` refuses a width
  or precision over 99. Opening another library, or another string function, means asking
  this question of it first.
- **`me:say` is capped** at `script.MaxSaysPerCall` (2) per call and
  `script.MaxSayLength` (300) bytes. Each say is a `Send` to everyone in the room, and a
  connection whose queue fills is hung up on, so a say in a loop would disconnect the
  room inside the deadline.
- **Each program has its own globals**: a copy of the base functions and of the
  `string`/`table`/`math` tables, with `_G` pointing at itself, and strings' shared
  metatable is locked. A script can break itself, not another script.

Adding a hook: a `Runtime` method that calls `fire` with its name, the name in `hooks`
(script/program.go), the Go call site, and a test in `world/scripts_test.go`.

### Definition vs Instance

The core modeling split, mirrored in `object` and `mobile`: a `Definition` is the loaded
template owned by a `Zone`; an `Instance` is a live thing with a `uuid.UUID`, pointing at its
definition. `ordered.List[K, T]` is the shared container for instances in inventories and
rooms.

**`ordered.List` is the one container.** A room's mobs, a room's floor, the players in a
room and a player's inventory are all the same thing: a map for lookup by id plus a slice
for the order to show the player. Ranging a map gives neither, which is how `look`
reshuffled the room and `kill lizard` picked a different lizard each round. `thing.Map`
(the pre-generics version, keyed on `IdStr`) is gone. What wraps it now:

- **`object.List`** is every container of objects: a room's floor (`Room.Inventory`), a
  player's inventory (`Player.Inventory()`) and a container's `Contents`. One type, so
  **moving an object between places is `object.Move(inst, from, to)`** -- `get`, `get
  from` and `drop` all use it, and a bag or `give` should too. It removes before it adds,
  and puts the object back if the add fails, so a failed move loses nothing. Don't write
  a remove-then-add by hand. **A floor lists newest first** (`object.NewFloor`, which
  `spaces.Room` uses): the order is the place's, not the caller's, so whatever lands on
  the floor -- a drop, a corpse, a zone reset, a wizard `load` -- goes on top. Bare
  `corpse` is the one that just fell and `2.corpse` the one before, the same order `look`
  shows. Inventories and container contents stay oldest first.
- **`player.List`** keys on `player.NameKey`, so a name typed in any case finds the player.
- **A room's mobs** are a bare `ordered.List` inside `spaces.Room`: a room is the only
  place mobs are listed, and `Occupancy` is the only writer.

The key is a `func(T) K` passed to `NewList`, not a constraint on `T`: `mobile.Instance`
has an `Id()` method and `object.Instance` has an `Id` field, and a field cannot satisfy a
method constraint. Pass a method expression (`(*mobile.Instance).Id`) or a closure.
`Find`/`FindAll` are free functions rather than methods because a method cannot introduce
the `Matcher` constraint, and constraining `List` itself would shut out `*player.Player`,
which has no `Matches`. `Add`/`Remove` return `ErrDuplicate`/`ErrNotFound` to wrap, since a
generic container cannot name what it is holding -- wrap with `%w` and the instance's name.

Prefer `All()` (an `iter.Seq`) over `Slice()`; the latter copies, and exists for callers
that index or hold the result. `object.List` returns its errors, wrapped with the object's
name: with one list per place and `Move` between them, a duplicate or a miss is a bug.
`player.List` logs them instead, because its callers are void methods deep inside a move
with nothing to do about it.

**`spaces.Room` is the one place this pattern was not applied** -- it holds static topology
and live contents in one struct, which is why `Room.Connect` has to be exported for the
loader. See ROADMAP.md "Known problems"; don't "fix" it piecemeal.

Location is tracked by **`spaces.Occupancy`**, not a pointer on the entity. A room keeps
its own lists of players and mobs (every `look` starts from a room, and wants them in a
stable order); `Occupancy` keeps the reverse index, player -> room and mob -> room, and is
the only code that writes either side. The room's writers (`addPlayer`, `playerEnters`,
`mobileLeaves`, ...) are unexported, so from `world/` a half-done move fails to compile
instead of leaving a ghost. `World.movePlayer` / `moveMobile` / `RemovePlayer` are one
`Occupancy` call each. It also lists every mob in the order it was placed
(`Mobiles()`), which is the order the mobile pulse wanders, aggroes and regenerates them
in -- a map range made that different every pulse. Objects are not in it: where an object is forms a tree (floor,
inventory, equipment, container) and nothing asks the reverse question yet.

A fight has no location of its own. Nobody can leave a fight without ending it (`move`
and `recall` refuse, `flee` ends it first), so `DoViolence` reports each swing to
`World.roomOf(fighter)` -- wherever the fighter is standing now.

### Combat

**Armor class is one absolute scale, `rules.BaseArmorClass` (10) at the bottom.** A hit is
`d20 + modifiers >= the defender's AC` (`combat.AttemptMeleeAttack`) -- meeting it is a
hit, not exceeding it. A player's AC is the baseline plus what `armor.json` says the worn
pieces are worth (`object.Equipment.ArmorClass`); a mob's is whatever `"ac"` in its
mobs.json says, on the same absolute scale, and the loader defaults a mob that doesn't say
to the baseline. It is a pointer in `loader.mobEntry` so that an absent key and an explicit
`"ac": 0` stay distinguishable: zero means a thing a d20 can never miss, and that is what
every mob missing the key silently used to be. Nothing displays AC to the player yet.

`combat` splits its interfaces by lifetime, not by entity:

- `Attacker` and `Defender` are the roles in a **single swing**. They swap every round, so
  nothing durable lives there. `melee.go` works only in these terms.
- `Combatant` is the **entity that persists across rounds** -- `Attacker` + `Defender` +
  `Id() uuid.UUID`, `Dead()`, `TakeMeleeDamage()`. The ledger and `World.DoViolence` work
  in these terms.

`combat.FightLedger` holds `Fight` records keyed on `uuid.UUID`. `Fight(A, B)` writes *two*
entries, `A->B` and `B->A`, so an attacker can be killed mid-round by their own target --
which is why `DoViolence` checks `Fighter.Dead()` before letting anyone swing. Both
`*player.Player` and `*mobile.Instance` satisfy `Combatant`, which is what keeps the melee
code from knowing which it is hitting.

**Fights happen in play now.** `kill` starts one, an `"aggressive"` mob starts one on the
mobile pulse with the first player in its room, and `DoViolence` carries it to a death.
Aggro skips a wizard with `nohassle` on -- and goes for the next player instead, so a
wizard shields nobody. `nohassle` is in memory only, switched on for every wizard's
session by `World.Arrive`, and only counts alongside the wizard bit; turn it off to test
aggro on your own character. It covers aggro and nothing else: a mob you attack fights
back.

**Targeting is "whoever engaged first", and it survives a death.** `Fight` never
overwrites a combatant's existing target, so a mob stays on whoever hit it first -- which
is what a tank relies on. `EndAllFightsWith` (death, flee, logout) then *retargets*: anyone
left being fought but no longer fighting turns on their earliest remaining attacker,
by the ledger's `seq`, since a map has no order. Without that the Barrow-King kills the
tank and stands there, still "in a fight" so aggro skips him, never swinging again. There
is no threat yet -- nothing lets a tank take a mob back.

## Conventions

- Handlers are one file each, `world/h_<command>.go`, with a sibling `h_<command>_test.go`.
  Wizard/builder commands are `h_wiz_<command>.go`, and **the command struct must
  implement `command.Wizard`** (`func (X) wizard() {}` beside the others in
  command/commands.go) and join the list in `world/wizard_test.go`. That marker is
  the only gate: `HandleIncomingMessage` refuses a marked command from anyone whose
  record lacks `Wizard`, before any handler runs. Forget it and the command is open to
  every player. Grant it with `make wizard NAME=...`, while they're logged out.
  `Bot` on the record is the same kind of hand-set flag (`make bot NAME=...`); it only
  labels the character in `who`, and nothing may branch on it.
- **`Send` returns nothing.** `player.Sender` is `Send(msg any)`. The only error any
  implementation could produce meant "this connection is already dead and I already tore it
  down," which no caller can act on. Don't reintroduce an error return.
- Tests use testify (`assert`, `require`, and `suite.Suite` for stateful ones). Fakes live in
  non-`_test.go` files so other packages can use them: `world.NewTestWorld()` (exported for
  exactly that reason), `player.NewTestPlayer`, `player.Recorder`, `gameserver.NewTestConn`,
  `spaces.NewTestRoom`, `combat.NewTestCombatant`, `rules.NewTestCatalog`. Assert on what a
  player received via the recorder, with the `sent[T]` helper in `world_test_helpers.go`:
  `sent[event.Dropped](s.T(), s.r, 0)`.
- `telnet/render_test.go` drives command strings through `NewTestWorld` and asserts on
  rendered output text -- parser, dispatch and renderer in one pass, no socket. Add a case
  there when you add a command.
- Target strings from the client are parsed by `world.parseTarget`: `all`, `all.knife`,
  `2.knife`, `20 coins`. Every command now carries the raw string and lets the handler call
  it -- the second grammar (`get`'s old pre-parsed `FindMode`/`Index`/`Target`) went away
  with protobuf, along with `message.FindMode`.
- Logging is mixed: newer code uses zerolog (`github.com/rs/zerolog/log`), older code the
  stdlib `log`. Follow whichever the file already uses.
- Config is `app.local.yaml` (`-config` to override, `-content` overrides just the content
  path). `serverconfig.Load` uses `yaml.UnmarshalStrict`, so an unknown key is a hard startup
  failure -- add the struct field and the YAML key in the same change. `deploy/app.yaml`
  is the container's, and `serverconfig/config_test.go` loads it, so it breaks `make
  check` rather than a deploy. `WATCHMUD_MONGO_URI` overrides `mongo.uri`: the uri holds
  the password and doesn't belong in a checked-in file. Never log it raw --
  `mongostore.RedactURI`. An empty `log.file` means stdout only, which is what the
  container uses.
- Deploying is `deploy/compose.yaml` and `deploy/README.md`; the root
  `docker-compose.yml` is only the unauthenticated dev mongo. Production runs only
  images built by `.github/workflows/build.yaml` from a `vX.Y.Z` tag on a `release/`
  or `hotfix/` branch; `master` is development. `deploy/deploy.sh` is how a version
  reaches the droplet. Rules and zones are in the image: content changes are released
  like code.
- `site/` is www.watchmud.com: GitHub Pages, published by `.github/workflows/pages.yaml`
  on any master push that touches it -- no release needed, since it's how to play, not
  the game. One self-contained `index.html` (the Windows/Mudlet guide). DNS has `www` as
  a CNAME to `watchmud.github.io`; the bare domain stays on the droplet for telnet. Keep
  its commands and wording in step with `help` when either changes.
