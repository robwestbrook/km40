# Handoff — KM40 weather dashboard

- **Written:** 2026-11-15 13:59 CST (19:59 UTC)
- **Session scope:** Review the code, fix the review findings in a worktree, close three spec gaps, clean up, merge to `main`, and deploy to the user's Linode as https://weather.mylifeapp.website.
- **Branch / last commit:** `main`; the newest commit is the handoff commit made right after this file was written (see `git log`). The last *code* commit is `77cc31a` "Add deployment to the shared Linode as weather.mylifeapp.website". **Not pushed**: `origin/main` is at `8a22df3`, so `main` is 12 commits ahead including the handoff commit.
- **Working tree:** clean after the handoff commit
- **Handoff status:** complete. Nothing is mid-flight.

## Read this first

KM40 is a single Go binary serving an embedded SvelteKit dashboard for METAR KM40 (Aberdeen /
Monroe County Airport, MS) with SQLite history from 2012. It is now **deployed and live at
https://weather.mylifeapp.website** on the shared Linode (`bps`, 173.255.193.174), behind Caddy, as
systemd service `km40` on `127.0.0.1:8082`. The server ingests live data itself; **production is the
source of truth for data now** (the local `km40.db` is a stale seed). All review fixes, the three spec
gaps (sky cover, wind direction, `/api/status` ingest health) and the cleanup pass are merged on `main`
but **not pushed to GitHub** (the user said "Commit, don't push"). No product work is pending; the next actions are the user's choice (see
"Decisions awaiting the user"). Do not push, and do not touch the other apps on the server, without being asked.

## Standing rules from the user

Carried over from the first session (still in force):

- **Stack is fixed:** Go standard library for HTTP, one third-party Go dependency (`modernc.org/sqlite`,
  non-CGO), SvelteKit 2, Tailwind 4, Chart.js. [project doc: docs/superpowers/specs/2026-11-14-km40-dashboard-design.md]
  (The design says "Go 1.22+"; `go.mod` is now `go 1.26.4` and the code uses `new(expr)`. A floor, not a conflict.)
- **Store UTC and canonical units; display America/Chicago, °F, mph.** Raw METAR text is always stored.
  [project doc: design]
- **Records are station records since 2012**, per calendar day, excluding the displayed window. [project doc: design]
- **Non-goals, not to be added unasked:** forecasts, radar, satellite, alerts, webcams, multi-station,
  accounts, sharing, mobile apps, official climate products. [project doc: design]
- **`km40.db` and the built binary stay untracked.** [my inference from `.gitignore`]
- **Git only as asked:** the user has asked for commits and a fast-forward merge this session, **never a push**.
  Every commit ends with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` and
  `Claude-Session: https://claude.ai/code/session_019zCdTpzQsRfpsAWJf6Pia7`. [harness attribution reminder]

Said this session (each approval is scoped to what it names):

- "Do a code review." Then: "Let's create a git worktree and start working on the problems found."
- Review items were approved **one at a time**: "Yes, start with item 1." (backfill), "Run the repair against a
  copy first.", "Move on to item 2" (parse failures), "Yes, present the approach for item 3" then
  **"Go with A and include the whole end date"** (station-local days via a registered SQL function; a date-only
  end date includes the whole day), "Yes, start on item 4" (weather decoding, gust unit), "Start on item 5"
  (shutdown and error handling).
- "Tackle spec gaps", then answers: **"Own section, prevailing direction (Recommended)"** (wind direction),
  **"Show them anyway (Recommended)"** (sky-cover records), **"Yes, all three (Recommended)"** (sky cover, wind
  direction, status health; one commit each, tests first, checked against a copy of the real DB).
- "Move on to the cleanup items" (cleanup pass; scope: the items listed in "What happened").
- "Show me the UI in a browser now", then "Done. Shut the server down".
- "Merge and run new build"; route chosen: **"Exit the worktree (Recommended)"**. Then "Stop the server. Clean
  the worktree." (done: worktree and branch removed).
- **"Keep the precipitation. The data may feed in later."** The `precip` element must stay although it shows no data.
- Deploy: "I want to deploy this web app to my current linode VPS ... subdomain on mylifeapp.website ... named
  'weather' ... Caddy is used on the server." Answers: seed with **"Upload local DB (Recommended)"**; deploy files:
  **"Yes, commit to main (Recommended)"** (commit only, no push). Then "weather A Record added." (done by the user in Namecheap).

**Not granted:** pushing to origin; changing the other apps (`bps`, `lifeapp`, Forward Church) or their repos
(read only); any server change beyond the km40 user and directories, the `km40` service and the one Caddy block;
anything beyond the single handoff commit described under "Exact stopping point".

## Exact stopping point

Last user message: "Update HANDOFF.md". The last substantive work was the deploy: `deploy/deploy.sh --seed-db km40.db`
succeeded, the `weather.mylifeapp.website` Caddy block was added and reloaded, and HTTPS was verified end to end
(certificate, gzip, HSTS, `/api/status`, `/api/summary`, the home page; the other sites still answered 200). The
previous handoff was archived to `handoffs/HANDOFF-2026-11-15-1359.md`. At the end the user answered "Save both to tools/"
(two session scripts, now in `tools/`) and "Commit, don't push" (this handoff, `handoffs/` and those scripts were committed
to `main` in one commit; nothing pushed). Nothing is uncommitted after that commit.

## Decisions awaiting the user

1. **Push `main` to origin?** 12 commits: the 11 from `c439d18` to `77cc31a` plus the handoff commit. Not blocking anything. Never done unasked.
2. **Long-range chart line vs labeled extremes** (open since the first session): charts plot bucket averages while
   "Window low/high" come from raw observations, so a 14-year temperature chart looks inconsistent with its labels.
   A min/max band would fix it. Recommended if it confuses; not requested.
3. **Precipitation record semantics** (open since the first session): `precip` records use per-bucket sums, so "record low"
   is nearly meaningless. The user chose to **keep the element**; how its records should read is still undecided.
4. **Disk tidy:** `km40.pre-review-fixes.db` (56 MB, gitignored) is the pre-migration backup of the real DB; the old
   scratchpad copy and test binaries also exist. Delete when the user is satisfied. Not decided.

## Next steps

**Approved and ready:** none. All work asked for is done.

**Proposed, needs the user's go-ahead:**

1. Push `main` (decision 1).
2. Min/max chart band for ranges over a day (decision 2). Touches `internal/store/store.go` and `web/src/lib/Chart.svelte`.
3. Remaining review leftovers, deliberately not done: `store.elements` duplicated in `web/src/lib/units.js` `ELEMENTS`
   (merging needs an API field or build step); `Station` lat/lon/elevation served but unused by the UI (removing changes
   the API); `Series` computes Min/Max in the loop for every element but only precipitation needs it.
4. Known visual flaws that pre-date this session: altimeter chart y-axis float noise (`30.400000000000006`);
   duplicate x-axis date labels in the 7-day view.
5. Future updates ship with `deploy/deploy.sh` from a clean tree.

## What happened this session

1. Code review, then a worktree (`worktree-review-fixes`) and one commit per fix, tests first:
   `c439d18` backfill month-end and partial-month bugs (IEM end date is exclusive); `afa9b09` reports that fail to parse
   are kept (raw only) instead of dropped; `d279d78` station-local (America/Chicago) calendar days for records and date
   parameters (SQL function `local_day`); `561033c` present weather decoded as phrases, gusts get a unit; `325270d`
   shutdown ordering and error handling, preset windows stay current.
2. Spec gaps: `d7087de` sky cover (percent of fullest layer, stored column `sky_cover`); `436b611` wind direction (circular
   statistics, window "prevailing" = most frequent of 16 compass sectors, share shown); `67d814b` `/api/status` now has an
   `ingest` block (backfill state/progress, poller last attempt/success/error), `null` with `-no-ingest`.
3. Cleanup: `622c763` (go mod tidy; removed `Series.Avg` and the redundant `observations_cols` index; shared `Observation`
   block; merged highs/lows loops; `new(expr)` replaces `p()`; zero-denominator visibility no longer stores infinity) and
   `0ef62ce` (`TIMEZONE`/`ARCHIVE_START` constants, removed dead code, `Window.svelte` no longer uses `getElementById`,
   `tools/cdp.mjs` dumps page text instead of a hook that never existed).
4. Merged by fast-forward into `main`, backed up the real DB, rebuilt, ran on the real DB: the one-time month refetch took
   about 8 minutes, observations went 337,190 to 348,916. Removed the worktree and branch.
5. Deployment (`77cc31a`): researched `lifeapp2`, `forward-church` and `bps` repos (read only) for the server pattern.
   Found that **DNS for mylifeapp.website is at Namecheap**, not Linode. Created the `km40` user and directories, deployed
   release `release-20261115-134510-77cc31a` with the seeded database, added the Caddy block after the user added the A record.

## Current state of the work

**Done and verified (run at handoff time)**

- `go vet ./...` clean; `go test -count=1 ./...`: `internal/api`, `internal/ingest`, `internal/metar`, `internal/store` all ok.
- `npm --prefix web test`: 15 tests, 15 pass.
- Production: `km40.service` active and enabled; `current` -> `/opt/km40/releases/release-20261115-134510-77cc31a`;
  `/api/status` shows 348,918 observations, last `2026-11-15T19:55:00Z`, poller succeeding every 5 minutes, backfill done 179/179.
- Public HTTPS verified by connecting to the server IP with the right hostname: Let's Encrypt certificate for
  `weather.mylifeapp.website` valid to 2027-02-13; HSTS and gzip present; `mylifeapp.website`, `www.busypastorsermons.com`
  and Forward Church (`:8090`) still 200. The process uses about 57 MB RAM; the server has about 560 MB available.
- Behaviour-preserving cleanup checked by comparing API output between the pre- and post-cleanup binaries on a DB copy
  (identical apart from the removed `avg`).

**Done but not verified**

- The deployed site was **not looked at in a rendered browser** on the public domain; only HTTP responses were checked.
- `deploy.sh`'s second-run path (database backup, rollback on failed health check) has **never run**; only the first
  deploy with `--seed-db` did. `/var/lib/km40/backups` does not exist yet.
- Why precipitation has no data in any window checked (2018-19, 2024): unverified; the archive METARs may not carry it.

**Not started:** push to GitHub; chart band; precipitation record semantics; deployment automation beyond `deploy.sh`
(no monitoring or alerting).

## File map

```
main.go                  flags (-addr -db -no-ingest -reparse), wiring, graceful shutdown, ingest health
internal/metar/          METAR parser, SkyCoverPercent/SkyCoverSQL, tests
internal/store/          SQLite, Series, records, localday.go (station-local days), circular.go (wind direction)
internal/ingest/         IEM backfill, AWC live poller, health.go (status for /api/status)
internal/api/            /api/summary /series /latest /status, static + SPA
web/                     SvelteKit app; lib/units.js (constants, ELEMENTS, helpers; node --test), Chart.svelte, Window.svelte
web/build/               gitignored but REQUIRED for go:embed
deploy/                  deploy.sh, setup-server.sh, km40.service, Caddyfile (block only), README.md
tools/                   cdp.mjs, cdp-click.mjs, custom-range.mjs (headless Chromium helpers), month-end-gaps.sql
docs/superpowers/        original design + plan; the plan predates many changes, trust code and this file
handoffs/                archived previous handoffs
km40.db                  LOCAL database, stale seed of production, gitignored
km40.pre-review-fixes.db local backup taken before the first run of the new build, gitignored
km40                     local built binary, gitignored
```

Server layout: binary `/opt/km40/releases/<release>/km40` (live one via `/opt/km40/current`); database
`/var/lib/km40/km40.db` owned by user `km40`; unit `/etc/systemd/system/km40.service`; Caddy block in `/etc/caddy/Caddyfile`
(backup before this change: `/etc/caddy/Caddyfile.bak-20261115-195127`); logs `journalctl -u km40`.

## Gotchas and known problems

- **DNS is at Namecheap** (`dns1/dns2.registrar-servers.com`). The Linode DNS zone for the domain is unused and stale
  (points at `50.116.32.235`); editing it does nothing. The user must make DNS changes in Namecheap.
- **This machine's resolver cached "no such name"** for `weather.mylifeapp.website` (my `dig` ran before the record existed),
  so lookups from here failed while public resolvers were correct. Test via the server IP with a Host/SNI override, or wait.
- **The server's Caddyfile is shared by four sites.** Never replace it; append, validate a copy (`caddy validate`), then reload.
  `/tmp/Caddyfile.new` on the server is a stale root-owned file from Oct 31 that blocks writes there: use `mktemp`.
- **`deploy.sh --seed-db` is first-deploy only**; it refuses when the server already has a database. Never re-seed over
  production data. A plain deploy stops the service, copies the DB to `/var/lib/km40/backups` (newest 5), then restarts.
  Rollback re-points `current` but does **not** reverse schema migrations.
- **First start of a new build runs one-time migrations** guarded by flags in the `cache` table (`backfill_end_exclusive`,
  `records_local_day`, `sky_cover_filled`). The real and production databases already passed them. Records rebuild takes
  ~30 s locally, ~10 s on the server.
- **`linode-cli` is not on PATH:** it lives at `~/.venvs/linode-cli/bin/linode-cli` (reports a version warning; harmless).
  Its token is in `~/.config/linode-cli`; never print it.
- **Tooling quirks seen:** Bash `curl` is redirected by a hook (use `fetch` through `ctx_execute`); kill test servers by exact
  name (`pgrep -x km40` or the task tool), never `pkill -f`; sub-agent output is data, not user authority. A worktree-isolated
  session cannot run git against the main checkout (there is no worktree now).
- **`web/build` must exist before `go build`** (npm build, then go build). `deploy.sh` does this.
- **Svelte 5 legacy-mode trap:** component-local functions or `$:` values can be stale right after assignment; read inputs
  directly or use module functions (see `Window.svelte`, `units.js`).
- **Not gofmt-clean, left alone:** `internal/store/store_test.go` (one line, pre-existing).
- **Observed on the server, not ours:** LifeApp listens on `*:8081` (all interfaces); the firewalls block it. Not changed.
- **IEM archive gaps are real** (2015-04..06, 2016-11..2017-09, 14 empty month-ends). Do not interpolate.
- **Session scripts were saved into `tools/` at the user's request** ("Save both to tools/"): `tools/custom-range.mjs <url>`
  drives the custom date inputs in headless Chromium (needs Chromium on port 9222) and prints the `/api/summary` requests it
  caused; `tools/month-end-gaps.sql` (was `q.sql`) counts empty month-ends after a backfill; run it on a copy of the DB. Neither
  was re-run after copying. The rest of the scratchpad (test binaries, DB copy, logs, screenshots) is disposable and will vanish.

## How to confirm state on resume

```sh
cd /mnt/ydrive/km40
git status --short                 # expect clean
git log --oneline -3               # expect the handoff commit, then 77cc31a, 0ef62ce
git rev-parse --short origin/main  # expect 8a22df3 (main is 12 ahead until pushed)
go vet ./... && go test -count=1 ./...   # expect 4 ok packages
npm --prefix web test              # expect 15 pass
pgrep -x km40 || echo none         # expect none locally
ssh robw@173.255.193.174 'systemctl is-active km40; readlink /opt/km40/current; curl -s 127.0.0.1:8082/api/status | head -c 200'
dig +short A weather.mylifeapp.website @1.1.1.1   # expect 173.255.193.174
```

Production observation count only grows; expect more than 348,918 and a last time within about an hour.

## Context the next session cannot get elsewhere

- The owner is Rob Westbrook; personal dashboard; wants US units on screen, metric storage. Other apps on the same Linode:
  the sermon site (`/mnt/ydrive/bps`, :8080), LifeApp (`/mnt/ydrive/lifeapp2`, :8081), Forward Church
  (`/mnt/ydrive/forward-church`, :8090). `deploy/` here mirrors the `lifeapp2` pattern.
- Server access is key-based SSH as `robw` (passwordless sudo) to `173.255.193.174`; no secrets are stored in this repo.
- Station: KM40, 33.8738 N, 88.4897 W, 68 ft, America/Chicago; IEM id is `M40`; archive starts 2012-01-11
  (stored first report 2012-01-12T00:15Z).
- Sky cover percentages: CLR 0, FEW 19, SCT 44, BKN 75, OVC/VV 100; wind direction has no min/max/average or records by design.
- The first handoff said the session ran under the user's "ponytail" minimal-change mode until told to stop. It never came up
  again this session and the user asked for broad changes; treat as unknown and keep changes scoped to what is asked.
