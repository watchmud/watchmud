#!/bin/sh
# Moves this host to a released version:
#
#   deploy/deploy.sh v1.2.3        # asks first: it disconnects everyone
#   deploy/deploy.sh -y v1.2.3     # doesn't ask
#
# The version is a git tag, and an image of the same name that
# .github/workflows/build.yaml built from it. Both move together: the checkout
# goes to the tag, so compose.yaml and the scripts are the release's too, and
# the game runs that tag's image -- code, rules and zones as released.
#
# Everything that can fail is checked before anything stops: the tag exists,
# the checkout has no local edits, the image pulls. Rolling back is deploying
# the previous version the same way.
set -eu

yes=
if [ "${1:-}" = "-y" ]; then yes=1; shift; fi
version="${1:?usage: deploy/deploy.sh [-y] <version tag, like v1.2.3>}"
case "$version" in
  v[0-9]*) ;;
  *) echo "deploy: $version isn't a version tag (v1.2.3)" >&2; exit 1 ;;
esac

cd "$(dirname "$0")/.."
env_file=deploy/.env
compose="docker compose -f deploy/compose.yaml"
[ -f "$env_file" ] || { echo "deploy: no $env_file -- see deploy/README.md, First time" >&2; exit 1; }
running=$(sed -n 's/^WATCHMUD_VERSION=//p' "$env_file")

git fetch --tags --quiet origin
if ! git rev-parse -q --verify "refs/tags/$version^{commit}" >/dev/null; then
  echo "deploy: there's no tag $version on origin" >&2; exit 1
fi
# .env, backups/ and certs/ are ignored; anything else changed here would be
# carried along or lost by the checkout
if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "deploy: this checkout has local changes; not deploying over them:" >&2
  git status --short >&2; exit 1
fi

echo "deploy: ${running:-nothing} -> $version"
# what to put back if this stops before the restart: otherwise the checkout and
# .env would name the new version while the old one runs, and the next
# `compose up`, for any reason, would quietly deploy it
previous_head=$(git rev-parse HEAD)
previous_env=$(mktemp)
cat "$env_file" > "$previous_env"
put_back() {
  git -c advice.detachedHead=false checkout --quiet "$previous_head"
  cat "$previous_env" > "$env_file"
  rm -f "$previous_env"
  echo "deploy: put the checkout and $env_file back; still running ${running:-what was running}" >&2
}
git -c advice.detachedHead=false checkout --quiet "$version"

# WATCHMUD_VERSION in .env, so every later compose command -- logs, restart --
# runs the same version this one did
tmp=$(mktemp)
grep -v '^WATCHMUD_VERSION=' "$env_file" > "$tmp" || true
echo "WATCHMUD_VERSION=$version" >> "$tmp"
cat "$tmp" > "$env_file" # keeps the file's owner and mode
rm -f "$tmp"

# before anything stops: an image that isn't there fails here, with the old
# version still running
if ! $compose pull --quiet watchmud; then
  echo "deploy: couldn't pull the image for $version; nothing was restarted." >&2
  put_back; exit 1
fi

if [ -z "$yes" ]; then
  printf 'deploy: restarting the game disconnects everyone playing. Go ahead? [y/N] '
  read -r answer
  case "$answer" in
    y|Y|yes) ;;
    *) echo "deploy: not restarted." >&2; put_back; exit 1 ;;
  esac
fi
rm -f "$previous_env"

$compose up -d --remove-orphans
$compose ps
echo "deploy: $version is running"

# The smoke test: the bot, from this version's image, logs in as a character
# made by hand once (README, "Smoke test") and walks, fights and loots. A
# failure leaves the new version running -- rolling back disconnects everyone
# again, and that is a person's call.
bot_password=$(sed -n 's/^WATCHMUD_BOT_PASSWORD=//p' "$env_file")
bot_name=$(sed -n 's/^WATCHMUD_BOT_NAME=//p' "$env_file")
if [ -z "$bot_password" ]; then
  echo "deploy: no WATCHMUD_BOT_PASSWORD in $env_file; not smoke testing (deploy/README.md, Smoke test)" >&2
  exit 0
fi
# the password reaches docker through the environment, not the command line
if ! WATCHMUD_BOT_PASSWORD="$bot_password" $compose run --rm --no-deps \
    -e WATCHMUD_BOT_PASSWORD --entrypoint /app/watchmud-bot \
    watchmud -addr watchmud:4000 -name "${bot_name:-Tester}"; then
  echo "deploy: $version is running but the smoke test FAILED (above)." >&2
  echo "deploy: to roll back: deploy/deploy.sh ${running:-<the previous version>}" >&2
  exit 1
fi
echo "deploy: smoke test passed"
