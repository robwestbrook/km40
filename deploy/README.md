# Deploying KM40

KM40 runs on the shared Linode (`bps`, 173.255.193.174) as a systemd service, `km40`, listening on
`127.0.0.1:8082`. Caddy terminates TLS and proxies **https://weather.mylifeapp.website** to it. The
sermon site (8080), LifeApp (8081) and Forward Church (8090) are untouched.

| Thing | Where |
|-------|-------|
| Binary (one per release) | `/opt/km40/releases/release-<time>-<sha>/km40`, live one at `/opt/km40/current` |
| Database | `/var/lib/km40/km40.db` (owned by the `km40` user) |
| Database backups | `/var/lib/km40/backups/km40-<time>.db`, newest five |
| Unit file | `/etc/systemd/system/km40.service` (copied from `deploy/km40.service`) |
| Caddy | `weather.mylifeapp.website` block in `/etc/caddy/Caddyfile` (see `deploy/Caddyfile`) |
| Logs | `journalctl -u km40` |

## One-time setup

1. `deploy/setup-server.sh` creates the `km40` user and directories.
2. **DNS is at Namecheap, not Linode** (the Linode zone for the domain is unused). Add an `A` record:
   host `weather`, value `173.255.193.174`.
3. `deploy/deploy.sh --seed-db km40.db` installs the first release and copies the local database up so
   the site has its history immediately. Without `--seed-db` the app backfills from IEM itself, which
   takes about 8 minutes.
4. Once `dig +short weather.mylifeapp.website` returns the server's IP, append the block in
   `deploy/Caddyfile` to `/etc/caddy/Caddyfile` (back it up first), then
   `sudo caddy validate --config /etc/caddy/Caddyfile && sudo systemctl reload caddy`.
   Do not replace the file: it serves the other sites too.

## Deploy

```
deploy/deploy.sh
```

Needs a clean git tree. Builds, uploads a new release, backs up the database, restarts, checks
`/api/status`, and points `current` back at the previous release if the check fails. Migrations are
not reversed by a rollback; restore the backup if a release changed the schema.

## Roll back by hand

```
ssh robw@173.255.193.174 'ls -1t /opt/km40/releases'
ssh robw@173.255.193.174 'ln -sfn /opt/km40/releases/<older> /opt/km40/current && sudo systemctl restart km40'
```

## Check

```
ssh robw@173.255.193.174 'systemctl status km40 --no-pager; curl -s 127.0.0.1:8082/api/status'
```
