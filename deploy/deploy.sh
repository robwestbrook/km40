#!/usr/bin/env bash
# Deploy KM40 to the shared Linode ("bps"), served at https://weather.mylifeapp.website.
#
# Usage:
#   deploy/deploy.sh                         # deploy the committed HEAD
#   deploy/deploy.sh --seed-db km40.db       # first deploy only: also install this database
#   KM40_HOST=user@host deploy/deploy.sh     # host override (default below)
#
# What it does, in order:
#   1. Refuses to run with uncommitted changes, so every release matches a commit.
#   2. Builds the web app and the linux/amd64 binary (pure Go, no CGO).
#   3. Uploads the binary to /opt/km40/releases/<release> and installs the unit file.
#   4. With --seed-db, installs the given database (only if the server has none).
#      Otherwise stops the service and keeps a copy of the live database in
#      /var/lib/km40/backups (newest five kept).
#   5. Points /opt/km40/current at the release and restarts the service.
#   6. Checks /api/status on the server. On failure it points current back at the
#      previous release. Database migrations are not reversed: restore the backup if
#      one is needed.
# Keeps the five newest releases. Run deploy/setup-server.sh once before the first deploy.
set -euo pipefail

HOST="${KM40_HOST:-robw@173.255.193.174}"
DOMAIN="weather.mylifeapp.website"
PORT=8082
REMOTE="/opt/km40"
DATA="/var/lib/km40"

SEED_DB=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed-db) SEED_DB="${2:?--seed-db needs a database path}"; shift 2 ;;
    *) echo "usage: deploy/deploy.sh [--seed-db path/to/km40.db]" >&2; exit 2 ;;
  esac
done

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHA="$(git -C "$REPO_ROOT" rev-parse --short HEAD)"
STAMP="$(date +%Y%m%d-%H%M%S)"
RELEASE="release-$STAMP-$SHA"
BUILD_DIR="$(mktemp -d)"
trap 'rm -rf "$BUILD_DIR"' EXIT

if [[ -n "$(git -C "$REPO_ROOT" status --porcelain)" ]]; then
  echo "error: uncommitted changes. Commit them first so the release matches a commit." >&2
  git -C "$REPO_ROOT" status --short >&2
  exit 1
fi

if [[ -n "$SEED_DB" ]]; then
  [[ -f "$SEED_DB" ]] || { echo "error: $SEED_DB not found" >&2; exit 1; }
  # A non-empty -wal file means the database was not closed cleanly; copying only the
  # main file would lose its newest rows.
  if [[ -s "$SEED_DB-wal" ]]; then
    echo "error: $SEED_DB-wal is not empty; stop the app that is using the database first." >&2
    exit 1
  fi
  if ssh "$HOST" "sudo test -e $DATA/km40.db"; then
    echo "error: the server already has a database; refusing to overwrite it." >&2
    exit 1
  fi
fi

ssh "$HOST" "test -d $REMOTE/releases" || {
  echo "error: server not prepared. Run deploy/setup-server.sh first." >&2
  exit 1
}

echo "==> Building web app and binary ($RELEASE)"
npm --prefix "$REPO_ROOT/web" run build >/dev/null
(cd "$REPO_ROOT" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="-s -w" -o "$BUILD_DIR/km40" .)

echo "==> Uploading to $HOST"
ssh "$HOST" "install -d -m 755 $REMOTE/releases/$RELEASE"
rsync -az "$BUILD_DIR/km40" "$HOST:$REMOTE/releases/$RELEASE/km40"
ssh "$HOST" "chmod 755 $REMOTE/releases/$RELEASE/km40"
rsync -az "$REPO_ROOT/deploy/km40.service" "$HOST:/tmp/km40.service"
ssh "$HOST" "sudo install -m 644 /tmp/km40.service /etc/systemd/system/km40.service && rm /tmp/km40.service \
  && sudo systemctl daemon-reload && sudo systemctl enable km40 >/dev/null 2>&1"

if [[ -n "$SEED_DB" ]]; then
  echo "==> Installing the seed database"
  rsync -az "$SEED_DB" "$HOST:/tmp/km40-seed.db"
  ssh "$HOST" "sudo install -o km40 -g km40 -m 640 /tmp/km40-seed.db $DATA/km40.db && rm /tmp/km40-seed.db"
else
  echo "==> Backing up the database"
  # Stop first: a clean shutdown checkpoints the WAL, so copying the one file is safe.
  ssh "$HOST" "sudo systemctl stop km40 2>/dev/null || true
    if sudo test -e $DATA/km40.db; then
      sudo install -d -o km40 -g km40 -m 750 $DATA/backups
      sudo cp -p $DATA/km40.db $DATA/backups/km40-$STAMP.db
      sudo bash -c 'cd $DATA/backups && ls -1t km40-*.db | tail -n +6 | xargs -r rm -f'
    fi"
fi

PREV="$(ssh "$HOST" "readlink $REMOTE/current" || true)"
echo "==> Switching current to $RELEASE and restarting"
ssh "$HOST" "ln -sfn $REMOTE/releases/$RELEASE $REMOTE/current && sudo systemctl restart km40"

echo "==> Checking /api/status"
ok=0
for _ in $(seq 1 15); do
  if ssh "$HOST" "curl -fsS http://127.0.0.1:$PORT/api/status >/dev/null"; then
    ok=1
    break
  fi
  sleep 2
done
if [[ "$ok" -eq 0 ]]; then
  echo "==> health check failed; rolling back to ${PREV:-nothing}" >&2
  ssh "$HOST" "sudo journalctl -u km40 -n 15 --no-pager" >&2 || true
  if [[ -n "$PREV" ]]; then
    ssh "$HOST" "ln -sfn $PREV $REMOTE/current && sudo systemctl restart km40" || true
  else
    ssh "$HOST" "sudo systemctl stop km40" || true
  fi
  exit 1
fi

# Not fatal: before the DNS record and the Caddy block exist, this cannot pass.
if ssh "$HOST" "curl -fsS --resolve $DOMAIN:443:127.0.0.1 https://$DOMAIN/api/status >/dev/null" 2>/dev/null; then
  echo "==> https://$DOMAIN answers through Caddy"
else
  echo "==> note: https://$DOMAIN does not answer through Caddy yet (DNS record or Caddy block missing)" >&2
fi

echo "==> Pruning old releases (keeping 5)"
ssh "$HOST" "cd $REMOTE/releases && ls -1dt release-* | tail -n +6 | xargs -r rm -rf"

echo "==> Deployed $RELEASE"
