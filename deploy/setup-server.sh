#!/usr/bin/env bash
# One-time server preparation for KM40 on the shared Linode. Safe to re-run.
#
# Usage: deploy/setup-server.sh            # host from KM40_HOST or the default below
#
# Creates the unprivileged `km40` service user, its data directory
# (/var/lib/km40, where the database lives) and the release directory
# (/opt/km40/releases, writable by the SSH user so deploys need no root).
# It does not touch Caddy, the firewall or any other app.
set -euo pipefail

HOST="${KM40_HOST:-robw@173.255.193.174}"

ssh "$HOST" 'sudo bash -s' <<'EOF'
set -euo pipefail
admin="${SUDO_USER:?run through sudo as the admin user}"
id km40 >/dev/null 2>&1 || useradd --system --home-dir /var/lib/km40 --shell /usr/sbin/nologin km40
install -d -o km40 -g km40 -m 750 /var/lib/km40
install -d -o "$admin" -g "$admin" -m 755 /opt/km40 /opt/km40/releases
echo "km40 user and directories ready"
EOF
