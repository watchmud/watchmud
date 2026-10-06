# Deploying WatchMUD

One Docker host running three containers: the game, its mongo, and a backup job.
Everything below runs from the repo root on that host.

Tested end to end with Docker 29, 2026-09-25: create a character, restart the game
container with that player still connected, log back in in the same room; backups
written and restored; mongo not reachable from the host; a session over TLS, and a
renewed certificate picked up with a player connected, who stayed connected.

## First time

You need Docker with the compose plugin, a checkout of this repo on the host, and a
domain name pointing at the host for the TLS certificate -- production is
`watchmud.com`: telnet on 4000, TLS on 4443. Get the certificate first
(see TLS, below): the game won't start with TLS configured and no certificate.

Nothing is built on the host. It runs images that GitHub Actions built from a version
tag (see Releasing, below), so there has to be one first.

```sh
cp deploy/.env.example deploy/.env
# two long random passwords, and the Spaces settings (Backups, below):
openssl rand -base64 24 | tr -d '/+='
git fetch --tags && git checkout v0.1.0      # deploy.sh needs to exist in the checkout
deploy/deploy.sh v0.1.0
telnet <host> 4000
```

Open ports 4000 and 4443 in the host's firewall, 80 for certbot's renewals, and
nothing else except ssh. Mongo publishes no port, so it is only reachable from the
other two containers. **Docker writes its own
iptables rules for published ports and goes around `ufw`**: a `ufw deny 4000` won't
close the game port. Use the cloud provider's firewall, or take the port out of
`compose.yaml`.

## TLS

Port 4443 is the same game over TLS, so passwords don't cross the internet in the
clear; port 4000 stays plain telnet for clients that can't do TLS, and tells players
4443 exists. Mudlet has a "Secure" checkbox, TinTin++ has `#ssl`, and anything else
can use `openssl s_client -connect <host>:4443`.

The certificate is Let's Encrypt's, through certbot on the host (not in a container).
certbot's standalone mode answers the challenge on port 80 itself:

```sh
sudo certbot certonly --standalone -d watchmud.com \
  --deploy-hook "$PWD/deploy/certbot-hook.sh"
```

The hook copies the certificate into `deploy/certs`, owned by the game's user (uid
65532 -- `/etc/letsencrypt` is root-only). certbot remembers the hook, and its
renewal timer runs it again every time the certificate is renewed. **The game
reloads the renewed certificate on the next connection, without a restart**, so
renewals don't disconnect anyone. The log says `tls: loaded the renewed certificate`.

Check it from anywhere:

```sh
openssl s_client -connect watchmud.com:4443 </dev/null 2>/dev/null | openssl x509 -noout -dates
```

## Making yourself a wizard

`make wizard` talks to the development mongo, which has no password. Here you go
through the root user. Log out of the game first, or the next timed save writes the
old value back.

```sh
docker compose -f deploy/compose.yaml exec mongo mongosh -u root -p \
  --authenticationDatabase admin watchmud \
  --eval 'db.players.updateOne({name: "Bob"}, {$set: {wizard: true}})'
```

It asks for the password (`MONGO_ROOT_PASSWORD`). The name is capitalized the way the
game stores it: `Bob`, not `bob`.

What a wizard can do in the game, beside playing:

| | |
|---|---|
| `gecho <words>` | say something to everyone -- "restarting in five minutes" before a deploy |
| `users` | who's playing, where, and who's muted or frozen |
| `mute <name>`, `freeze <name>` | a player can't talk to anyone; can only look and quit. Again to undo. Kept on their record |
| `goto <zone/room or name>`, `transfer <name>` | go somewhere; bring someone to you |
| `purge [thing]`, `zreset [zone]` | clear the room; reset a zone now |
| `reports` | the latest `bug`/`idea`/`typo` reports since the restart (all of them: see "Reports") |
| `slay`, `restore`, `load`, `gold`, `nohassle`, `roomstatus`, `echo` | the older tools |

Every one is logged at warn with `commandType=wiz`.

## Releasing

`master` is development. A release is a `release/X.Y` branch cut from it, and each
version a tag on that branch; fixes to something released go on a `hotfix/` branch
cut from its tag. Content is released the same way: rules and zones are in the image,
so a version is the code and the world together.

```sh
# a new release
git switch -c release/0.2 master && git push -u origin release/0.2
git tag -a v0.2.0 -m "v0.2.0" && git push origin v0.2.0

# a fix to what's running
git switch -c hotfix/0.2.1 v0.2.0
# ... fix, commit ...
git push -u origin hotfix/0.2.1
git tag -a v0.2.1 -m "v0.2.1" && git push origin v0.2.1
# and merge the fix back into master (and the release branch)
```

Pushing the tag runs `.github/workflows/build.yaml`: the tests, then the image
`ghcr.io/watchmud/watchmud:v0.2.0`. It refuses a version tag that isn't on a
`release/` or `hotfix/` branch. `gh run watch` follows it. Pushes to master and to
the branches build images too (`:master`, `:release-0.2`, `:sha-abc1234`), for trying
things -- production only ever runs a version.

The package has to be **public** for the droplet to pull it without logging in: after
the first image, the watchmud organization's Packages, `watchmud`, Package settings,
Change visibility. (Or keep it private and `docker login ghcr.io` on the droplet with
a token that can only read packages.)

## Deploying

On the droplet:

```sh
cd /srv/watchmud
deploy/deploy.sh v0.2.0
```

It fetches the tag, checks the checkout has no local edits, moves the checkout to the
tag (so compose.yaml and the scripts are that release's), records the version in
`.env`, and pulls the image -- all before anything stops. If any of that fails, it puts
the checkout and `.env` back and the running version carries on. Then it asks, and
restarts, and waits for the game's health check to pass (below, "Health") -- up to two
minutes, or it stops there and says how to roll back. It ends with the smoke test
(below, "Smoke test").

**Restarting disconnects everyone.** The game gets SIGTERM, saves everyone who is
logged in, and exits; compose waits up to 30s (`stop_grace_period`) before it would
SIGKILL. Players reconnect to wherever they were. Announce it first.

Rolling back is deploying the previous version: `deploy/deploy.sh v0.1.0`. The log's
first line says what's running: `WatchMUD starting version=v0.2.0`.

Old images pile up on a small disk: `docker image prune` now and then.

**Never `--build` on the droplet.** `deploy/compose.build.yaml` is for trying the stack
on your own machine.

## Smoke test

After every restart `deploy.sh` runs `watchmud-bot` from the image it just deployed.
The bot logs in, recalls to Temple Square, walks south to the millpond, fights the
geese, loots a corpse, drops the feather, and recalls before it quits -- so next time
it logs in to Temple Square, not beside geese that have respawned. It prints a line
per step. A failure prints everything it saw and exits non-zero, **and leaves the new
version running**: rolling back is `deploy/deploy.sh <previous version>`, by hand, because it
disconnects everyone a second time.

"no goose at the millpond" is a pass: a player got there first.

It needs a character, made once by hand -- the bot never creates one, since names are
permanent:

1. `telnet watchmud.com 4000`, make a character (`Tester`, or anything else), `quit`.
2. In `deploy/.env`:

   ```
   WATCHMUD_BOT_PASSWORD=<its password>
   WATCHMUD_BOT_NAME=Tester        # only if it isn't Tester
   ```

Without `WATCHMUD_BOT_PASSWORD` the deploy skips the smoke test and says so. From a
laptop, against any server: `WATCHMUD_BOT_PASSWORD=... bin/watchmud-bot -addr host:4000`.

## Bots

The `bots` service plays a few characters around the clock: they walk out to the
Hollowfields, fight what's a fair fight, loot, rest, and leave what they find in the
donation room. They're listed apart, under Bots, in `who`, `help bots` explains them, and a
tell to one gets an automatic honest answer. They leave a hunting ground to any player
they meet there.

Each bot is a character made once by hand -- the bots never create one:

1. `telnet watchmud.com 4000`, create it (`Wren`, say), `quit`. Same password for all.
2. Flag it, while it's logged out, so `who` says what it is:

   ```sh
   docker compose -f deploy/compose.yaml exec mongo mongosh -u root -p \
     --authenticationDatabase admin watchmud \
     --eval 'db.players.updateOne({name: "Wren"}, {$set: {bot: true}})'
   ```

   Do the same for the smoke test's character (Tester): it's a bot too.
3. In `deploy/.env`: `WATCHMUD_BOTS=Wren,Pim` (hunters), `WATCHMUD_WANDERERS=Odo`
   (wanderers: they roam the safe parts of the world, linger, fight only back and
   take nothing), `WATCHMUD_SOCIALITES=Mabel` (one is plenty: it stands in Temple
   Square welcoming new characters and answering questions), `WATCHMUD_EXPLORERS=...`
   (explorers: they walk every exit they safely may until they've mapped it all,
   then wander) and `WATCHMUD_BOTS_PASSWORD=...`, then
   `docker compose -f deploy/compose.yaml up -d bots`. Any list can be empty; a name
   in two stops the service at startup. Five bots at most, all lists together.

At most 5: they all connect from the one container, and the game allows 5
connections per address. `docker compose -f deploy/compose.yaml logs -f bots` shows
what they're up to, a line per change of plan. Every deploy restarts them onto the
new version; they log back in within a couple of minutes.

## Logs

```sh
docker compose -f deploy/compose.yaml logs -f watchmud
```

Docker keeps them and rotates them: 5 files of 20MB per container.

The game logs where each connection came from (`telnet 1.2.3.4:5678`). After the
first real players connect, **check those are their addresses**. If every
connection comes from the same address (a gateway, a proxy), the 5-per-address
connection cap is counting that address, and the sixth player is refused.
(IPv6 counts by /64.) There's also a cap of 200 connections in all
(`telnet.maxConns`): past it, "The game is full right now." A login must be over
within 5 minutes of connecting.

## Health

The game answers `GET /healthz` on 127.0.0.1:4080 inside the container (`health:` in
`app.yaml`): 200 while its loop has ticked in the last 30 seconds, 503 once it hasn't.
That is the failure a crash-restart can't see -- the world goroutine wedged while the
listeners still accept connections nobody will ever answer. compose.yaml's healthcheck
runs `watchmud -healthcheck` every 10s (the image has no curl), and the port isn't
published.

```sh
docker compose -f deploy/compose.yaml ps        # (healthy) / (unhealthy)
docker compose -f deploy/compose.yaml exec watchmud /app/watchmud -config /app/app.yaml -healthcheck
```

**Unhealthy is not restarted for you.** Docker only restarts a container that exits.
A restart disconnects everyone, but a wedged world already has: look at the logs (a
panic in a pulse is recovered and logged, a hang isn't), then `docker compose -f
deploy/compose.yaml restart watchmud`. The bots wait for healthy before they start.

**From outside**, `.github/workflows/uptime.yaml` checks every 15 minutes that both ports
get an answer from the world -- not just the banner, which the connection writes on its
own and a wedged loop would still print -- by giving a name, `Uptimeprobe`, and hanging
up at "Create them?". It also fails when the TLS certificate has under 10 days left. A
failed run is GitHub emailing you; three tries 20s apart keep a deploy's restart from
setting it off. It runs from master only. Never create `Uptimeprobe`: the check would
still pass (a taken name asks for a password), but there's no reason to.

## Reports

Players file `bug`, `idea` and `typo` reports in the game. Each is a warn-level log
line (`report=bug player=... room=zone/room`) and a document in mongo's `reports`
collection; a wizard reads the latest 50 since the restart with `reports`. To read
them all:

```sh
docker compose -f deploy/compose.yaml exec mongo mongosh -u root -p \
  --authenticationDatabase admin watchmud --quiet \
  --eval 'db.reports.find().sort({at: -1}).limit(50)'
```

## Backups

The `backup` service writes `deploy/backups/watchmud-<time>.archive.gz` when it starts
and every night after, and deletes ones older than two weeks. Those are on the same
disk as the database: they cover a bad deploy or a mistake, not losing the droplet.

The `offsite` service covers that. Every hour it copies new dumps to a DigitalOcean
Spaces bucket, which keeps them for 90 days. It copies and never syncs, so the droplet
deleting its old dumps doesn't delete them from the bucket.

### Setting up the bucket

1. In the DigitalOcean console, create a Spaces bucket -- say `watchmud-backups` --
   in a **different region from the droplet** (nyc3; the droplet is in sfo3), with
   file listing restricted. The name is global across Spaces, so it may need a suffix.
2. Create a Spaces access key **limited to that bucket**, with read/write/delete
   (delete is how the 90-day pruning works). The secret is shown once.
3. **Before pulling a version of this repo that has the `offsite` service**, add the
   settings to `deploy/.env` -- compose refuses to run anything, even `logs`, while
   they're missing:

   ```sh
   SPACES_REGION=nyc3
   SPACES_BUCKET=watchmud-backups
   SPACES_KEY=<access key>
   SPACES_SECRET=<secret>
   ```

4. Then start it. Only the new service starts; the game isn't touched:

   ```sh
   git pull
   docker compose -f deploy/compose.yaml up -d offsite
   docker compose -f deploy/compose.yaml logs offsite
   ```

   The log should say `Copied (new)` for each dump and `offsite: copied to
   spaces:watchmud-backups/backups`. Check the files are in the bucket in the console.

### What each copy survives

| Copy | Survives | Doesn't survive |
|---|---|---|
| `deploy/backups` (nightly, 2 weeks) | a bad deploy, a mistake in the data | losing the droplet |
| Spaces bucket (hourly, 90 days, nyc3) | losing the droplet, or sfo3 | root on the droplet: the key is in `.env` and can delete, since that's how the pruning works |
| DigitalOcean droplet backups (console, Backups tab) | root on the droplet -- nothing on it can delete them | losing the DigitalOcean account |

The game itself can't reach the key: it runs as a non-root user in a distroless
container with no shell, and the key is only in `.env` and the `offsite` container.
Getting it takes root on the host.

Keep **2FA on the DigitalOcean account**: it's the one place all three can be deleted
from.

A droplet backup restores the whole droplet (Backups tab, restore or create a droplet
from it). For just the characters, the dumps are quicker.

### Restoring

Restoring replaces the game's data with what's in the file, so stop the game first:

```sh
docker compose -f deploy/compose.yaml stop watchmud
docker compose -f deploy/compose.yaml exec -T mongo mongorestore -u root -p <root password> \
  --authenticationDatabase admin --gzip --archive --drop < deploy/backups/<file>
docker compose -f deploy/compose.yaml start watchmud
```

If the droplet is gone, the dump comes from the bucket. Download it in the console, or
on a new droplet with the same `.env`:

```sh
docker compose -f deploy/compose.yaml run --rm -v "$PWD/deploy/backups:/restore" \
  --entrypoint rclone offsite copy spaces:watchmud-backups/backups/<file> /restore
```

then restore it as above.

## Passwords

The game's database password is set when mongo's volume is created, by
`mongo-init.js`, and never again. Changing `WATCHMUD_DB_PASSWORD` in `.env` later
just stops the game connecting. To change it, change it in mongo too:

```sh
docker compose -f deploy/compose.yaml exec mongo mongosh -u root -p \
  --authenticationDatabase admin watchmud \
  --eval 'db.changeUserPassword("watchmud", "<new password>")'
# then set it in .env, and:
docker compose -f deploy/compose.yaml up -d watchmud
```

## Not done yet

- **Nothing acts on unhealthy.** The health check (above) notices a hang, and the uptime
  workflow emails about it; a person restarts it. An autoheal sidecar, if restarts
  become routine, is later.
