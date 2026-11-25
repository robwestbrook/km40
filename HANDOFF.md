# Handoff — KM40 weather dashboard

- **Written:** 2026-11-25 14:30 CST (20:30 UTC); the previous handoff is archived as `handoffs/HANDOFF-2026-11-25-1419.md` (older ones: `handoffs/HANDOFF-2026-11-15-*.md`).
- **Session scope:** Resume from the handoff, then add a compass-rose dial for wind direction (brainstorm, design, plan, build, review, merge, push, deploy), and clean up.
- **Branch / last commit:** `main`; the newest commit is this handoff (see `git log`). The last *code* commit is `4396259` "Add the compass dial to the wind direction detail page"; `f233f27` added the design, plan and mockups. **Pushed** to `origin/main` (`dffb2ee..f233f27`) and **deployed** as `release-20261125-121923-f233f27`.
- **Working tree:** clean after the handoff commit
- **Handoff status:** complete. Nothing is mid-flight.

## Read this first

KM40 is a single Go binary serving an embedded SvelteKit dashboard for METAR KM40 (Aberdeen / Monroe County Airport, MS) with
SQLite history from 2012. It is **live at https://weather.mylifeapp.website** on the shared Linode (`bps`, 173.255.193.174),
behind Caddy, as systemd service `km40` on `127.0.0.1:8082`, and is installed on the user's iPhone as a home-screen web app.
Production is the source of truth for data (the local `km40.db` is a stale seed). This session replaced the dot chart on the
Wind direction card with a compass dial (needle, wind speed in the lower third) and added the dial beside the dot chart on the
Wind direction detail page. It is deployed, and the user confirmed it on their phone: **"It works well. Approve this addition."**
No product work is pending. **Do not do any database work** (standing rule below), do not push or deploy again, and do not touch
the other apps on the server, unless asked.

## Standing rules from the user

Carried over (still in force):

- **Stack is fixed:** Go standard library for HTTP, one third-party Go dependency (`modernc.org/sqlite`, non-CGO), SvelteKit 2,
  Tailwind 4, Chart.js. [project doc: docs/superpowers/specs/2026-11-14-km40-dashboard-design.md]
  (The design says "Go 1.22+"; `go.mod` is `go 1.26.4` and the code uses `new(expr)`. A floor, not a conflict.)
- **Store UTC and canonical units; display America/Chicago, °F, mph.** Raw METAR text is always stored. [project doc: design]
- **Records are station records since 2012**, per calendar day, excluding the displayed window. [project doc: design]
- **Non-goals, not to be added unasked:** forecasts, radar, satellite, alerts, webcams, multi-station, accounts, sharing,
  native mobile apps, official climate products. [project doc: design] The installable web app and the compass dial were asked for.
- **Keep the precipitation element** although it shows no data: "Keep the precipitation. The data may feed in later." [said in an earlier session]
- **"Do not do any databse related work. We do not need it for building or tests."** [said in an earlier session; never lifted]
  Kept all of this session too: no database copied or opened, tests use none, and visual checks used a stub API. The deploy
  script's own on-server backup of the production DB ran as part of the approved deploy. Ask first if a task seems to need data.
- **`km40.db` and the built binary stay untracked.** [my inference from `.gitignore`]
- **Git only as asked.** Every approval covers only what it names:
  - Earlier sessions: "Push"; "Yes, fix the handoff and commit it, and push"; "commit, merge, push, and deploy."; "Clean up the
    worktree, update HANDOFF.md and any file that needs to update, commit, and push."; "Yes, update the handoff, commit and push".
  - **This session:** **"Commit, merge, push, and deploy. You will have to do this without input from me. I have an appointment."**
    Done once: docs committed, fast-forward merge, one push, one plain `deploy/deploy.sh`. And **"Yes"** to my offer to
    "update HANDOFF.md and remove the worktree" (the offer said updating it means a commit and a push), so **this handoff's one
    commit and one push** are approved; nothing beyond that.
  - Every commit ends with `Co-Authored-By: Claude Sonnet 5.5 <noreply@anthropic.com>` and a `Claude-Session:` line. Copy the
    session URL from the harness attribution reminder of the *current* session; do not type it from memory. This session's was
    `https://claude.ai/code/session_019zCdTp3A3dc3bhnCbetcsS`. [harness attribution reminder]

Said this session (each approval is scoped to what it names):

- "Start a new git worktree: wind-mod" (created, later removed). "Let's brainstorm changing the wind direction to a compass rose".
- Look of the dial: "The current direction as a needle, with the direction degrees numerically shown in the lower third"; then
  "Replace the wind heading with the wind speed."; then **"Your example has the wind speed in the upper left card corner and the
  wind direction still on the compass. Swap their positions"**; confirmed "That's correct." The settled design is: card upper
  left keeps the direction ("245 °" and "WSW"), and the **wind speed ("9 mph") sits in the lower third of the dial**. The degrees
  are *not* in the dial. (This supersedes the first request.)
- Home card layout "A" (dial replaces the dot chart): chosen by clicking "A". "**The detail page should keep the dot chart.**"
- "I approve the design. Add the superpowers folder to the git repository." I read this as `.superpowers/` (the mockups) and
  committed only its `content/` pages, **not** `.last-token` (a browser-session key) or `state/`. [my inference]
- "Write the plan"; "Approve plan. Use native" (I implement inline; I treated this as approval of the plan's local commits).
- "1" (let the user look at the preview), "I approve" (home card), "Looks good. Approve" (detail page).
- "What is the URL for the web app?" then "It works well. Approve this addition".
- "1" to deleting three stale server-leftover files so the worktree could be removed.

**Not granted:** pushes or deploys beyond those named above; changing the other apps (`bps`, `lifeapp`, Forward Church) or their
repos (read only); server changes beyond the km40 user and directories, the `km40` service and the one Caddy block; database work.

## Exact stopping point

Last user message: "1" (delete the leftovers, remove the worktree). Done: deleted `.last-port`, `.last-token` and
`state/server-stopped` from the worktree, `git worktree remove`, `git branch -d worktree-wind-mod` (it was merged at `f233f27`),
archived the old handoff, wrote this file. Next, in the same turn: commit this file and `handoffs/HANDOFF-2026-11-25-1419.md`,
then push, which the user approved.

Before that: after "commit, merge, push, and deploy" the sequence ran as `f233f27` pushed, `deploy.sh` finished with "Deployed
release-20261125-121923-f233f27" (its own health check and the Caddy check passed). I then fetched the live site from this machine:
`/api/status` returned 200 and the dial code ("Wind direction unavailable") was found in the live JS chunk `BJ0vmVmR.js`.
The user then opened the web app on their phone and approved it.

## Decisions awaiting the user

None block anything. Open since the first session, still undecided:

1. **Long-range chart line vs labeled extremes:** charts plot bucket averages while "Window low/high" come from raw observations,
   so a 14-year temperature chart looks inconsistent with its labels. A min/max band would fix it. Recommended if it confuses; not requested.
2. **Precipitation record semantics:** `precip` records use per-bucket sums, so "record low" is nearly meaningless. The user chose
   to keep the element; how its records should read is undecided.
3. **Disk tidy:** `km40.pre-review-fixes.db` (56 MB, gitignored) is the pre-migration backup of the real DB, and old scratchpad
   copies and test binaries also exist. Delete when the user is satisfied. It counts as database work, so ask first.

## Next steps

**Approved and ready:** none. All work asked for is done.

**Proposed, needs the user's go-ahead:**

1. Min/max chart band for ranges over a day (decision 1). Touches `internal/store/store.go` and `web/src/lib/Chart.svelte`.
2. Compass leftovers (all minor): the dial's `aria-label` rounds 359.6° to "360 degrees" (`web/src/lib/Compass.svelte`; needle is correct);
   an independent code review of the compass branch was never done (see Gotchas).
3. Older review leftovers, deliberately not done: `store.elements` duplicated in `web/src/lib/units.js` `ELEMENTS`; `Station`
   lat/lon/elevation served but unused; `Series` computes Min/Max for every element but only precipitation needs it.
4. Known visual flaws that pre-date this work: altimeter chart y-axis float noise (`30.400000000000006`); duplicate x-axis
   date labels in the 7-day view.
5. Future updates ship with `deploy/deploy.sh` from a clean tree (and, only if the user asks, `git push`).

## What happened this session

1. Resumed: read this handoff; tree clean, `main` level with `origin`, `go vet` and the four Go packages ok. (The check over ssh
   was refused by the auto-mode classifier at that point, so production health was not re-checked then.)
2. Brainstormed with the visual companion (browser mockups in `.superpowers/brainstorm/`): needle dial; layout A chosen; the
   speed/direction swap; detail page keeps the dot chart. Wrote a short design doc and a three-task plan.
3. Implemented natively in worktree `wind-mod` (TDD for the helpers): Task 1 `needleAngle(deg, speedKt)` and `speedText(speedKt)` in
   `web/src/lib/units.js` (3 tests, red then green, web tests 15 to 18); Task 2 `web/src/lib/Compass.svelte` and the home card; Task 3 the detail page.
4. Visual checks used a throwaway stub API plus `vite preview`, shown to the user in the companion tab; the user approved each.
5. Final review: the fresh reviewer agent failed (the model I picked needed usage credits), so I did a self-review. One minor found (aria-label).
6. Merged `worktree-wind-mod` into `main` (fast-forward), verified the merged tree, pushed, deployed, checked the live JS, user approved on their phone.
7. Cleanup: the worktree and branch removed, the old handoff archived, this handoff written.

## Current state of the work

**Done and verified**

- `go vet ./...` clean; `go test -count=1 ./...`: `internal/api`, `internal/ingest`, `internal/metar`, `internal/store` ok; `npm --prefix web test` 18/18. All run on the merged tree before the push.
- Deploy: `deploy.sh` completed and its health and Caddy checks passed; live `/api/status` 200; live JS contains the dial code.
- **User confirmed the compass on their phone** ("It works well").
- Browser-checked with the stub, user approved: the home card and `/element/winddir` with a WSW wind (245°, 8 kt = "9 mph").

**Done but not verified**

- The **calm, variable and no-data cases were never seen in a browser.** They are covered by `needleAngle`/`speedText` tests and code reading: calm or variable gives no needle, speed still shows; the parser stores a bearing only when speed is above 0 and the wind is not VRB (`internal/metar/metar.go`), so both arrive with a null bearing, and the card's upper left then shows "—" with "calm" or "variable" beneath.
- **Phone-width layout (375 px) was not checked in a browser here**; the user saw it on their phone and approved.
- **No independent code review** of the compass branch (self-review only).
- The `deploy.sh` rollback branch (failed health check) has still never run. `/var/lib/km40/backups` was not listed after the deploys.
- Why precipitation has no data in any window checked (2018-19, 2024): unverified; the archive METARs may not carry it.

**Not started:** chart band; precipitation record semantics; monitoring or alerting.

## File map

```
main.go                  flags (-addr -db -no-ingest -reparse), wiring, graceful shutdown, ingest health
internal/metar/          METAR parser, SkyCoverPercent/SkyCoverSQL, tests
internal/store/          SQLite, Series, records, localday.go (station-local days), circular.go (wind direction)
internal/ingest/         IEM backfill, AWC live poller, health.go (status for /api/status)
internal/api/            /api/summary /series /latest /status, static + SPA (staticHandler registers the .webmanifest MIME type)
web/                     SvelteKit app; lib/units.js (constants, ELEMENTS, helpers incl. NEW needleAngle/speedText; node --test), Chart.svelte, Window.svelte
web/src/lib/Compass.svelte   NEW: SVG dial (props bearing, speed, size); used on the home card and /element/winddir
web/src/routes/+page.svelte  home page; the winddir card shows Compass instead of Chart
web/src/routes/element/[key]/+page.svelte  detail page; for winddir, Compass beside the dot chart, no "from ..." line (Wind speed page keeps it)
web/src/app.html         page shell: manifest link, apple-touch-icon, iPhone meta tags
web/static/              manifest.webmanifest, apple-touch-icon.png, icon-192.png, icon-512.png
web/build/               gitignored but REQUIRED for go:embed
deploy/                  deploy.sh, setup-server.sh, km40.service, Caddyfile (block only), README.md
tools/                   make-icons.py, cdp.mjs, cdp-click.mjs, custom-range.mjs (headless Chromium helpers), month-end-gaps.sql
README.md                run, develop, layout; "Install on a phone" section
docs/superpowers/        original design + plan; NEW specs/2026-11-25-wind-compass-design.md and plans/2026-11-25-wind-compass.md
.superpowers/brainstorm/ NEW, tracked: the compass mockup pages (content/ only; session tokens and server state are never committed)
handoffs/                archived previous handoffs
km40.db                  LOCAL database, stale seed of production, gitignored (no database work)
km40.pre-review-fixes.db local backup taken before the first run of the new build, gitignored
km40                     local built binary, gitignored
```

Server layout: binary `/opt/km40/releases/<release>/km40` (live one via `/opt/km40/current`); database `/var/lib/km40/km40.db`
owned by user `km40`; unit `/etc/systemd/system/km40.service`; Caddy block in `/etc/caddy/Caddyfile` (backup before the first
deploy: `/etc/caddy/Caddyfile.bak-20261115-195127`); logs `journalctl -u km40`.

## Gotchas and known problems

- **The stub-API script is lost.** `stub-api.mjs` (a throwaway Node `http` server on `127.0.0.1:8080`, which `web/vite.config.js`
  proxies `/api` to) lived in the session scratchpad and was not saved into the repo. It served fixture JSON shaped like the real API
  (`/api/summary` = `{station, observedAt, ageMinutes, raw, current, from, to, elements:[Series]}`, `/api/latest`, `/api/series`;
  `Series` fields are in `internal/store/store.go`) and picked a wind case (WSW 245°/8 kt, calm, variable, none) from a file on each
  request. Rebuild it the same way if a DB-free visual check is needed. Offer to save a copy under `tools/` if wanted.
- **Browser tooling for screenshots is broken on this machine:** the `chrome-devtools` MCP looks for `/opt/google/chrome/chrome`
  (not installed); headless Brave (`/opt/brave.com/brave/brave`) hangs without writing a screenshot, even with `--headless=old`.
  What worked: serve the build with `vite preview` and let the user look. **`vite preview` crashes if you rebuild while it runs**
  (hashed files vanish): restart it. `vite dev` plus headless also hung.
- **Hooks and the classifier:** local `curl` is redirected by a hook (use `node -e fetch`, or `ctx_execute`); a read-only
  `ssh ... systemctl is-active km40` was refused by the auto-mode classifier before the deploy was authorized, while `deploy.sh`'s
  own ssh worked once the user said to deploy. A hook rewrites the prompt of `Agent` calls and the classifier then asks for the
  call to be reissued unchanged.
- **Review agent model:** the `Agent` call with `model: "fable"` failed ("requires usage credits"). Pick another model or do the review inline.
- **Worktree sessions:** the harness refuses compound shell commands and git against the main checkout from inside a worktree.
  Split commands, use `git -C <path>`, and use `ExitWorktree` (keep) before merging into `main`. A new worktree needs
  `npm --prefix web ci` and `npm --prefix web run build` before `go build` or `go test`.
- **A hand-typed `Claude-Session:` id was wrong once** this session (fixed by amending an unpushed commit). The id of a session can be
  found by grepping `~/.claude/projects/<project-dir>/<session>.jsonl` for `claude.ai/code/session_`, but prefer the attribution reminder.
- **iPhone install is manual** (Share, then Add to Home Screen). The page uses a `black-translucent` status bar with `viewport-fit=cover`
  and relies on `env(safe-area-inset-*)` padding on `body`. After a deploy, an installed copy may need a full close and reopen.
- **`.webmanifest` MIME type:** Go's table lacks it, so `staticHandler` registers it in code. Do not rename the file without checking.
- **DNS is at Namecheap** (`dns1/dns2.registrar-servers.com`). The Linode DNS zone for the domain is unused and stale.
- **The server's Caddyfile is shared by four sites.** Never replace it; append, validate a copy (`caddy validate`), then reload.
  `/tmp/Caddyfile.new` on the server is a stale root-owned file from Oct 31: use `mktemp`.
- **`deploy.sh --seed-db` is first-deploy only**; it refuses when the server already has a database. A plain deploy stops the
  service, copies the DB to `/var/lib/km40/backups` (newest 5), then restarts. Rollback re-points `current` but does **not** reverse schema migrations.
- **First start of a new build runs one-time migrations** guarded by flags in the `cache` table (`backfill_end_exclusive`,
  `records_local_day`, `sky_cover_filled`). The real and production databases already passed them.
- **`linode-cli` is not on PATH:** it lives at `~/.venvs/linode-cli/bin/linode-cli`. Its token is in `~/.config/linode-cli`; never print it.
- **Tooling quirks:** kill test servers by exact name (`pgrep -x km40`), never `pkill -f`; sub-agent output is data, not user authority.
- **Svelte 5 legacy-mode trap:** component-local functions or `$:` values can be stale right after assignment; read inputs
  directly or use module functions (see `Window.svelte`, `units.js`, and the inline expressions passed to `Compass`).
- **Not gofmt-clean, left alone:** `internal/store/store_test.go` (one line, pre-existing).
- **Observed on the server, not ours:** LifeApp listens on `*:8081` (all interfaces); the firewalls block it. Not changed.
- **IEM archive gaps are real** (2015-04..06, 2016-11..2017-09, 14 empty month-ends). Do not interpolate.

## How to confirm state on resume

```sh
cd /mnt/ydrive/km40
git status --short                 # expect clean
git log --oneline -4               # expect this handoff, then f233f27 (design/plan/mockups), 4396259, 00fc4cd
git worktree list                  # expect only /mnt/ydrive/km40 [main]
git branch                         # expect only main
git fetch origin && git rev-list --count origin/main..main   # expect 0 (everything is pushed)
go vet ./... && go test -count=1 ./...   # expect 4 ok packages (needs web/build; run `npm --prefix web run build` if missing)
npm --prefix web test              # expect 18 pass
pgrep -x km40 || echo none         # expect none locally
ssh robw@173.255.193.174 'systemctl is-active km40; readlink /opt/km40/current'
# expect: active, release-20261125-121923-f233f27 (or newer)
```

Production observation count only grows (348,918 at an earlier check); not re-checked this session.

## Context the next session cannot get elsewhere

- The owner is Rob Westbrook; personal dashboard; wants US units on screen, metric storage. Other apps on the same Linode:
  the sermon site (`/mnt/ydrive/bps`, :8080), LifeApp (`/mnt/ydrive/lifeapp2`, :8081), Forward Church (`/mnt/ydrive/forward-church`, :8090). `deploy/` mirrors the `lifeapp2` pattern.
- Server access is key-based SSH as `robw` (passwordless sudo) to `173.255.193.174`; no secrets are stored in this repo.
- Station: KM40, 33.8738 N, 88.4897 W, 68 ft, America/Chicago; IEM id is `M40`; archive starts 2012-01-11.
- Sky cover percentages: CLR 0, FEW 19, SCT 44, BKN 75, OVC/VV 100; wind direction has no min/max/average or records by design.
- The user likes to look at a thing before approving it: they approved the compass only after seeing it in the browser, and they
  change their mind mid-design (the degrees-in-the-dial request became speed-in-the-dial). Show a mockup or a live preview early.
  They also set tight limits on side effects and ask for work to be done without a design discussion when it is small.
  Keep changes scoped to what is asked and state any choice of mine in one line.
- The first handoff said the session ran under the user's "ponytail" minimal-change mode until told to stop. It has not come up since; treat as unknown.
