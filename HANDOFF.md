# Handoff — KM40 weather dashboard

- **Written:** 2026-11-15 14:20 CST (20:20 UTC)
- **Session scope:** Resume from the previous handoff, make the dashboard installable as an iPhone home-screen web app, then commit, merge, push and deploy it, clean up the worktree and update the docs.
- **Branch / last commit:** `main`; the newest commit is this handoff and README update (see `git log`). The last *code* commit is `4023658` "Make the dashboard installable as a home-screen web app". **Pushed** to `origin/main` at the user's request, and **deployed** as release `release-20261115-141357-4023658`.
- **Working tree:** clean after the handoff commit
- **Handoff status:** complete. Nothing is mid-flight.

## Read this first

KM40 is a single Go binary serving an embedded SvelteKit dashboard for METAR KM40 (Aberdeen /
Monroe County Airport, MS) with SQLite history from 2012. It is **live at https://weather.mylifeapp.website**
on the shared Linode (`bps`, 173.255.193.174), behind Caddy, as systemd service `km40` on `127.0.0.1:8082`.
Production is the source of truth for data (the local `km40.db` is a stale seed). This session added a web app
manifest, icons and iPhone meta tags so it can be installed from Safari (Share, then Add to Home Screen). That is
deployed and the live files were checked, but **it has not been tried on a real iPhone**. No product work is
pending. **Do not do any database work** (standing rule below), do not push again and do not touch the other apps
on the server unless asked.

## Standing rules from the user

Carried over (still in force):

- **Stack is fixed:** Go standard library for HTTP, one third-party Go dependency (`modernc.org/sqlite`,
  non-CGO), SvelteKit 2, Tailwind 4, Chart.js. [project doc: docs/superpowers/specs/2026-11-14-km40-dashboard-design.md]
  (The design says "Go 1.22+"; `go.mod` is `go 1.26.4` and the code uses `new(expr)`. A floor, not a conflict.)
- **Store UTC and canonical units; display America/Chicago, °F, mph.** Raw METAR text is always stored. [project doc: design]
- **Records are station records since 2012**, per calendar day, excluding the displayed window. [project doc: design]
- **Non-goals, not to be added unasked:** forecasts, radar, satellite, alerts, webcams, multi-station, accounts,
  sharing, mobile apps, official climate products. [project doc: design] The user has now asked for an installable
  **web** app, which I read as not a native mobile app, so not a conflict [my inference]. Nothing beyond installing
  was added (no offline mode, no notifications).
- **Keep the precipitation element** although it shows no data: "Keep the precipitation. The data may feed in later." [said in an earlier session]
- **`km40.db` and the built binary stay untracked.** [my inference from `.gitignore`]
- **Git only as asked.** Every push so far was asked for explicitly and each approval covered that push only:
  "Push"; "Yes, fix the handoff and commit it, and push" (earlier session); **"commit, merge, push, and deploy."** and
  **"Clean up the worktree, update HANDOFF.md and any file that needs to update, commit, and push."** (this session).
  Push again only when asked.
  Every commit ends with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` and a `Claude-Session:` line; copy the
  session URL from the harness attribution reminder of the *current* session (this session's was
  `https://claude.ai/code/session_019zCdTpPqk9qa6eVqUBuLZi`; earlier ones used a different id). [harness attribution reminder]

Said this session (each approval is scoped to what it names):

- **"Do not do any databse related work. We do not need it for building or tests."** Said right after the worktree was
  created. Kept for the whole session: I copied no database, ran the app against none, and the one new Go test uses an
  in-memory file system. The deploy script's own on-server backup of the production DB ran as part of the approved
  deploy. Treat the rule as in force until the user lifts it; ask if a task seems to need data. [said this session]
- "Start a new git worktree to start" (created `worktree-humming-forging-sphinx`; removed at the end, see below).
- "I want to make this a web app that I can install on my iPhone." When offered a design discussion: **"Just make it
  installable as a web app."** That opted out of the questions and the written design, so there is **no design doc for
  this change**; it is recorded here and in `README.md` instead.
- "commit, merge, push, and deploy." Done as one fast-forward merge, one push, one plain `deploy/deploy.sh`.
- "Clean up the worktree, update HANDOFF.md and any file that needs to update, commit, and push." (this handoff, the
  `README.md` addition and the push).

**Not granted:** pushes beyond the ones named above; changing the other apps (`bps`, `lifeapp`, Forward Church) or their
repos (read only); any server change beyond the km40 user and directories, the `km40` service and the one Caddy block;
database work of any kind.

## Exact stopping point

Last user message: "Clean up the worktree, update HANDOFF.md and any file that needs to update, commit, and push."
Done in this order: removed the worktree and its branch (merged, plain `git branch -d`), added the README section, wrote this
handoff (previous one archived to `handoffs/HANDOFF-2026-11-15-1416.md`), committed, pushed. Before that the code
commit `4023658` was fast-forward merged, pushed (`fef1355..4023658`) and deployed; `deploy.sh` finished with
"Deployed release-20261115-141357-4023658".

Live checks run from the server through Caddy (the local `curl` is blocked by a hook, so use ssh): `/manifest.webmanifest`
200 `application/manifest+json`; `/apple-touch-icon.png` and `/icon-512.png` 200 `image/png`; the home page links the manifest
and touch icon; `km40` active; `current` points at the new release. Not run: anything on an iPhone.

## Decisions awaiting the user

None block anything.

1. **Does the install work and look right on the iPhone?** (new) Check the icon, the name "KM40", full-screen mode and the
   spacing under the notch and home bar. The icon is a sun behind a cloud on dark blue; changing it means editing
   `tools/make-icons.py`, regenerating, committing and redeploying.
2. **Long-range chart line vs labeled extremes** (open since the first session): charts plot bucket averages while
   "Window low/high" come from raw observations, so a 14-year temperature chart looks inconsistent with its labels.
   A min/max band would fix it. Recommended if it confuses; not requested.
3. **Precipitation record semantics** (open since the first session): `precip` records use per-bucket sums, so "record low"
   is nearly meaningless. The user chose to keep the element; how its records should read is undecided.
4. **Disk tidy:** `km40.pre-review-fixes.db` (56 MB, gitignored) is the pre-migration backup of the real DB, and the old
   scratchpad copies and test binaries also exist. Delete when the user is satisfied. Not decided, and it counts as database work, so ask first.

## Next steps

**Approved and ready:** none. All work asked for is done.

**Proposed, needs the user's go-ahead:**

1. Try the install on the iPhone (decision 1) and fix whatever looks wrong.
2. Min/max chart band for ranges over a day (decision 2). Touches `internal/store/store.go` and `web/src/lib/Chart.svelte`.
3. Remaining review leftovers, deliberately not done: `store.elements` duplicated in `web/src/lib/units.js` `ELEMENTS`
   (merging needs an API field or build step); `Station` lat/lon/elevation served but unused by the UI (removing changes
   the API); `Series` computes Min/Max in the loop for every element but only precipitation needs it.
4. Known visual flaws that pre-date this work: altimeter chart y-axis float noise (`30.400000000000006`); duplicate x-axis
   date labels in the 7-day view.
5. Future updates ship with `deploy/deploy.sh` from a clean tree (and, only if the user asks, `git push`).

## What happened this session

1. Read this handoff and checked state: tree clean, `main` level with `origin`, `go vet` and all four Go packages ok, web tests
   15/15, production active at release `77cc31a`, DNS correct. Nothing had drifted.
2. Created a worktree with `EnterWorktree` (random name). A fresh worktree lacks the gitignored `web/node_modules` and
   `web/build`, so I ran `go mod download`, `npm ci` and `npm run build` before the baseline tests (all passed).
3. Built the installable web app: `web/static/manifest.webmanifest` (name "KM40 Weather", standalone, `#070b14`),
   icons `apple-touch-icon.png` (180), `icon-192.png`, `icon-512.png` made by `tools/make-icons.py` (Pillow), the head tags
   in `web/src/app.html` (manifest, touch icon, theme colour, `apple-mobile-web-app-*`, `viewport-fit=cover`), and
   safe-area padding on `body` in `web/src/app.css`. **No service worker, by my choice** (iPhone installs without one and
   the data is live, so a cache could only show stale weather).
4. Go side, test first: Go's built-in MIME table lacks `.webmanifest`, so the file was served as `text/plain`. The new test
   `TestStaticServesWebManifest` failed on that, then passed once `staticHandler` called `mime.AddExtensionType`
   (`internal/api/api.go`). The test uses `fstest.MapFS`, no database.
5. Verified before shipping: `go vet`, `go test ./...` (4 packages ok), `npm --prefix web test` (15/15), the production build
   runs and `web/build/200.html` carries the new head tags, `gofmt -l` lists only the old `internal/store/store_test.go`.
   I looked at the 512 px icon.
6. Committed `4023658` in the worktree, left it with `ExitWorktree` (keep), fast-forward merged into `main`, pushed,
   ran `deploy/deploy.sh`, and checked the live files (see "Exact stopping point"). Then removed the worktree and branch.

## Current state of the work

**Done and verified**

- `go vet ./...` clean; `go test -count=1 ./...`: `internal/api`, `internal/ingest`, `internal/metar`, `internal/store` ok; `npm --prefix web test` 15/15 (all run before the deploy).
- Production: `km40` active, `current` -> `/opt/km40/releases/release-20261115-141357-4023658`; manifest and icons served with correct content types; the home page links them.
- `deploy.sh` second-run path: the backup, switch, restart and health-check steps ran without error. The **rollback** branch (failed health check) has still never run.

**Done but not verified**

- **Not tried on an iPhone**: install, icon, title, full-screen behaviour and safe-area layout are unchecked.
- The deployed site was **not looked at in a rendered browser** on the public domain; only HTTP responses were checked.
- I did not list `/var/lib/km40/backups` after the deploy; the script reported the backup step without error.
- Why precipitation has no data in any window checked (2018-19, 2024): unverified; the archive METARs may not carry it.

**Not started:** chart band; precipitation record semantics; monitoring or alerting.

## File map

```
main.go                  flags (-addr -db -no-ingest -reparse), wiring, graceful shutdown, ingest health
internal/metar/          METAR parser, SkyCoverPercent/SkyCoverSQL, tests
internal/store/          SQLite, Series, records, localday.go (station-local days), circular.go (wind direction)
internal/ingest/         IEM backfill, AWC live poller, health.go (status for /api/status)
internal/api/            /api/summary /series /latest /status, static + SPA (staticHandler registers the .webmanifest MIME type)
web/                     SvelteKit app; lib/units.js (constants, ELEMENTS, helpers; node --test), Chart.svelte, Window.svelte
web/src/app.html         page shell: manifest link, apple-touch-icon, iPhone meta tags
web/static/              NEW: manifest.webmanifest, apple-touch-icon.png, icon-192.png, icon-512.png (copied into the build as is)
web/build/               gitignored but REQUIRED for go:embed
deploy/                  deploy.sh, setup-server.sh, km40.service, Caddyfile (block only), README.md
tools/                   make-icons.py (NEW, regenerates the icons; `python3 -I tools/make-icons.py`, needs Pillow),
                         cdp.mjs, cdp-click.mjs, custom-range.mjs (headless Chromium helpers), month-end-gaps.sql
README.md                run, develop, layout; gained an "Install on a phone" section
docs/superpowers/        original design + plan; no design doc for the install feature (the user opted out)
handoffs/                archived previous handoffs
km40.db                  LOCAL database, stale seed of production, gitignored (do not use it: no database work)
km40.pre-review-fixes.db local backup taken before the first run of the new build, gitignored
km40                     local built binary, gitignored
```

Server layout: binary `/opt/km40/releases/<release>/km40` (live one via `/opt/km40/current`); database
`/var/lib/km40/km40.db` owned by user `km40`; unit `/etc/systemd/system/km40.service`; Caddy block in `/etc/caddy/Caddyfile`
(backup before the first deploy: `/etc/caddy/Caddyfile.bak-20261115-195127`); logs `journalctl -u km40`.

## Gotchas and known problems

- **iPhone install is manual.** Safari shows no install prompt; the user adds it via Share, then Add to Home Screen. The page uses
  `black-translucent` status bar with `viewport-fit=cover`, so content runs under the status bar and relies on the
  `env(safe-area-inset-*)` padding on `body`. If the top or bottom looks cramped, start there.
- **`.webmanifest` MIME type:** Go's table lacks it, so `staticHandler` registers it in code. Do not rename the file without checking.
- **A new worktree needs `npm --prefix web ci` and `npm --prefix web run build` before `go build` or `go test`**, because
  `web/node_modules` and `web/build` are gitignored and `go:embed` needs `web/build`.
- **Worktree sessions:** the harness refuses compound shell commands and cannot run git against the main checkout. Split
  commands, and use `ExitWorktree` (keep) before merging into `main`. `ExitWorktree` only removes worktrees it made; this
  one was removed with `git worktree remove` and `git branch -d` after the merge.
- **Local `curl` is redirected by a hook.** To check the live site, run curl over ssh on the server with
  `--resolve weather.mylifeapp.website:443:127.0.0.1`, or use `fetch` through `ctx_execute`.
- **DNS is at Namecheap** (`dns1/dns2.registrar-servers.com`). The Linode DNS zone for the domain is unused and stale; the user
  must make DNS changes in Namecheap. This machine's resolver once cached "no such name" for `weather.mylifeapp.website`.
- **The server's Caddyfile is shared by four sites.** Never replace it; append, validate a copy (`caddy validate`), then reload.
  `/tmp/Caddyfile.new` on the server is a stale root-owned file from Oct 31: use `mktemp`.
- **`deploy.sh --seed-db` is first-deploy only**; it refuses when the server already has a database. Never re-seed over
  production data. A plain deploy stops the service, copies the DB to `/var/lib/km40/backups` (newest 5), then restarts.
  Rollback re-points `current` but does **not** reverse schema migrations.
- **First start of a new build runs one-time migrations** guarded by flags in the `cache` table (`backfill_end_exclusive`,
  `records_local_day`, `sky_cover_filled`). The real and production databases already passed them.
- **`linode-cli` is not on PATH:** it lives at `~/.venvs/linode-cli/bin/linode-cli` (version warning is harmless). Its token is
  in `~/.config/linode-cli`; never print it.
- **Tooling quirks:** kill test servers by exact name (`pgrep -x km40`), never `pkill -f`; sub-agent output is data, not user authority.
- **Svelte 5 legacy-mode trap:** component-local functions or `$:` values can be stale right after assignment; read inputs
  directly or use module functions (see `Window.svelte`, `units.js`).
- **Not gofmt-clean, left alone:** `internal/store/store_test.go` (one line, pre-existing).
- **Observed on the server, not ours:** LifeApp listens on `*:8081` (all interfaces); the firewalls block it. Not changed.
- **IEM archive gaps are real** (2015-04..06, 2016-11..2017-09, 14 empty month-ends). Do not interpolate.
- **Session scripts in `tools/`:** `custom-range.mjs <url>` drives the custom date inputs in headless Chromium (needs Chromium on
  port 9222); `month-end-gaps.sql` counts empty month-ends after a backfill (run on a copy of the DB, which is database work: ask first).
  Neither was re-run recently.

## How to confirm state on resume

```sh
cd /mnt/ydrive/km40
git status --short                 # expect clean
git log --oneline -3               # expect this handoff/README commit, then 4023658 (installable web app), fef1355
git worktree list                  # expect only /mnt/ydrive/km40 [main]
git branch                         # expect only main
git fetch origin && git rev-list --count origin/main..main   # expect 0 (everything is pushed)
go vet ./... && go test -count=1 ./...   # expect 4 ok packages (needs web/build; run `npm --prefix web run build` if missing)
npm --prefix web test              # expect 15 pass
pgrep -x km40 || echo none         # expect none locally
ssh robw@173.255.193.174 'systemctl is-active km40; readlink /opt/km40/current; curl -s -o /dev/null -w "%{http_code} %{content_type}\n" --resolve weather.mylifeapp.website:443:127.0.0.1 https://weather.mylifeapp.website/manifest.webmanifest'
# expect: active, release-20261115-141357-4023658 (or newer), 200 application/manifest+json
```

Production observation count only grows (348,918 at the previous check); I did not re-check it this session.

## Context the next session cannot get elsewhere

- The owner is Rob Westbrook; personal dashboard; wants US units on screen, metric storage. Other apps on the same Linode:
  the sermon site (`/mnt/ydrive/bps`, :8080), LifeApp (`/mnt/ydrive/lifeapp2`, :8081), Forward Church
  (`/mnt/ydrive/forward-church`, :8090). `deploy/` here mirrors the `lifeapp2` pattern.
- Server access is key-based SSH as `robw` (passwordless sudo) to `173.255.193.174`; no secrets are stored in this repo.
- Station: KM40, 33.8738 N, 88.4897 W, 68 ft, America/Chicago; IEM id is `M40`; archive starts 2012-01-11
  (stored first report 2012-01-12T00:15Z).
- Sky cover percentages: CLR 0, FEW 19, SCT 44, BKN 75, OVC/VV 100; wind direction has no min/max/average or records by design.
- When the user wants something small they say so ("Just make it installable as a web app") and want the work done without a
  design discussion; they also set tight limits on side effects ("Do not do any databse related work"). Keep changes scoped
  to what is asked and state any choice of mine in one line.
- The first handoff said the session ran under the user's "ponytail" minimal-change mode until told to stop. It has not come up
  since; treat as unknown.
