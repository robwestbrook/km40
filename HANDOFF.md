# Handoff — KM40 weather dashboard

- **Written:** 2026-11-14 20:55 CST
- **Session scope:** Build the KM40 dashboard end to end (Go + SQLite + SvelteKit), verify it in a browser, then publish it to GitHub.
- **Branch / last commit:** `main` @ `a5726ac` "KM40 weather dashboard: Go API, METAR ingest, SvelteKit UI" (pushed to origin)
- **Working tree:** clean before this handoff; uncommitted now: `HANDOFF.md`, `CLAUDE.md`, `tools/cdp.mjs`, `tools/cdp-click.mjs`
- **Handoff status:** complete

## Read this first

The project is a single Go binary that serves an embedded SvelteKit dashboard for METAR
**KM40** (Aberdeen / Monroe County Airport, Mississippi), backed by SQLite history from
2012 to now. It is built, tested, verified in headless Chromium, committed, and pushed to
`https://github.com/robwestbrook/km40.git`. A dev server is still running on
`127.0.0.1:8099` against the real 54 MB database (337,190 observations). There is no
functional work mid-flight: the only open items are optional visual/data-semantics
improvements, listed under "Next steps" and "Decisions awaiting the user". Do not commit
or push the handoff files until the user says so.

## Standing rules from the user

- **Stack is fixed:** Go 1.22+ with the standard library only for HTTP (no router or web
  framework), exactly one third-party Go dependency (`modernc.org/sqlite`, non-CGO),
  SvelteKit 2, Tailwind CSS 4, Chart.js. [project doc: docs/superpowers/specs/2026-11-14-km40-dashboard-design.md]
- **Approve-before-build was honored this way:** the user replied "1. Embedded static build.
  2. Backfill once. 3. Approved. Build as you have presented." — that is an approval of the
  design as written, not a blanket permission for new features. [said this session]
- **Records mean station records since 2012**, per calendar day (MM-DD), always excluding
  the window being displayed so a day is never compared with itself. [project doc: design]
- **Store UTC, display America/Chicago.** Store canonical units (°C, kt, mi, inHg, ft) and
  convert to °F/mph for display in the UI. [project doc: design]
- **Never lose a report:** raw METAR text is always stored next to the parsed columns, and
  `-reparse` must be able to rebuild history from raw text. [project doc: design]
- **Non-goals (do not add unasked):** forecasts, radar, satellite, alerts, webcams,
  multi-station support, accounts, sharing, mobile apps, official climate products.
  [project doc: design]
- **The user asked for git work explicitly:** "Init a git repo. Create a .gitignore and a
  README.md. Commit all files.", then "Run these commands: git branch -M main / git remote
  add origin ...", then "Push to origin". All three were done. [said this session]
- The database (`km40.db`, 54 MB) and the built binary are deliberately untracked; do not
  add them to git. [my inference from `.gitignore`]

## Exact stopping point

The last functional work was the element-page rewrite, the performance and correctness
fixes listed below, and a final verification pass. After that the user initialized git,
committed `a5726ac`, added the `origin` remote, and pushed; then invoked `/shutting-down`.
The app is running in the background as PID 255105:
`./km40 -addr 127.0.0.1:8099 -db /mnt/ydrive/km40/km40.db`. This handoff, `CLAUDE.md`,
and the two `tools/` scripts were just created and are **not committed**. Nothing else was
left half-done.

## Decisions awaiting the user

1. **Commit and push the handoff files?** `HANDOFF.md`, `CLAUDE.md`, `tools/cdp.mjs`,
   `tools/cdp-click.mjs` are uncommitted. Asked at the end of the session; the answer may
   arrive after this file is written. Not blocking any code work.
2. **Long-range chart line vs. labeled extremes.** Charts plot bucket averages while
   "Window low/high" now come from raw observations, so a 14-year temperature chart shows a
   line spanning ~43–85 °F next to labels of 5.0 / 109.4 °F. A min/max band would fix it;
   not built (not requested). Recommended if the user finds the discrepancy confusing.
3. **Precipitation records.** `precip` high/low use per-bucket sums of the per-report `Pxxxx`
   group, so "record low" (0.01 in) is close to meaningless. Options: show only totals for
   precip, or special-case its records. Not approved either way.

## Next steps

Approved and ready, nothing functional is pending.

Proposed, needs the user's go-ahead before doing:

1. Add a min/max band (or switch aggregation) for chart ranges longer than a day, so the
   line agrees with the labeled window extremes. Touches `internal/store/store.go`
   (`Series` would need per-bucket min/max) and `web/src/lib/Chart.svelte`.
2. Decide what "high/low/record" should mean for precipitation and adjust the card and
   element page accordingly.
3. Optional: stop the leftover dev server (`for p in $(pgrep -x km40); do kill $p; done`) —
   it is still running from this session.

## What happened this session

1. Wrote the design (`docs/superpowers/specs/2026-11-14-km40-dashboard-design.md`) and the
   implementation plan (`docs/superpowers/plans/2026-11-14-km40-dashboard.md`).
2. Built `internal/metar` (tolerant METAR parser, tests), `internal/store` (SQLite schema,
   upsert, bucketed series, per-day records, `Reparse`, records snapshot cache),
   `internal/ingest` (IEM month backfill, AWC live poller), `internal/api` (JSON endpoints,
   SPA fallback), `main.go` (flags `-addr`, `-db`, `-no-ingest`, `-reparse`), and the
   SvelteKit app under `web/`.
3. Ran the real one-time IEM backfill: 337,188 observations, 2012-01-12 → 2026-11-15.
   IEM gaps are real (2015-04…06 and 2016-11…2017-09 have few or zero reports).
4. Fixed the parser so an omitted wind group means calm (0 kt) while a masked `/////KT`
   stays unknown, then replayed all history with `-reparse` (337,188 rows).
5. Records rebuild was a 33 s full-table aggregate on this drive; added a JSON snapshot in
   the `cache` table (key `records`, ~3.2 MB) loaded at startup, with a background refresh.
6. Built and visually verified the dashboard and element pages with headless Chromium.
7. Found and fixed a Svelte 5 legacy-mode trap: a component-local function called from the
   template (`series()`, `spanHours()`) can keep seeing stale state; replaced with `$:`
   derived values. `onMount(load)` also failed to run the load on the element page; a
   reactive block keyed on `loadedKey` replaced it.
8. Rewrote the element page to use `GET /api/series?element=…` plus the new
   `GET /api/latest` instead of `/api/summary` (14-year "All" went from ~13 s to 1.6 s).
9. Chart labels now use time of day ≤ 72 h, month + day ≤ 90 days, month + year beyond;
   times that land days apart no longer render as clock times.
10. Business fixes: element page record cards show `—` when there is no record outside the
    window (they showed a fake 32.0 °F); window high/low now come from raw observations with
    the time they occurred (bucket averages understated raw extremes); record ties resolve to
    the earliest date so results are stable across restarts.
11. Performance: `/api/summary` runs its nine element queries concurrently (dashboard "All"
    13 s → 5.1 s); `/api/status` derives years from the observation span instead of a
    `DISTINCT strftime` full scan (2.1 s → 0.2 s).
12. Verified again in Chromium (dashboard 24 h and All, element temp 24 h and All, element
    wind) with no console errors or exceptions.
13. Initialized git on `main`, added `.gitignore` and `README.md` (both already existed from
    the build), committed 30 files as `a5726ac`, added the `origin` remote, and pushed.

## Current state of the work

**Done and verified**

- `go test -count=1 ./...` passes: `internal/api`, `internal/ingest`, `internal/metar`,
  `internal/store` (re-run at handoff time).
- Database: 337,190 observations, first `2012-01-12T00:15:00Z`, last `2026-11-15T02:35:00Z`,
  years 2012–2026, nine elements.
- API timings measured at handoff: `/api/latest` 2 ms, `/api/summary` (24 h) 11 ms,
  `/api/status` 0.2 s, 14-year single-element series 1.6 s, `/api/summary?from=2012-01-11`
  ~5.1 s.
- Real values spot-checked: all-time temp min −15 °C (5.0 °F) on 2018-01-17, max 43 °C
  (109.4 °F) on 2019-10-26 — the raw METAR for that reading is
  `KM40 261735Z AUTO 22017G27KT 10SM FEW017 SCT022 BKN048 43/37 A2966 RMK AO2`, so it is
  station data, not a parsing bug.
- UI checks done in headless Chromium against `127.0.0.1:8099`; screenshots were written to
  `/tmp` during the session and are gone.

**Done but not verified**

- Nothing functional. The only untested surface is the Svelte UI itself, which has no
  automated tests (browser checks were manual).

**Not started**

- Long-range min/max chart band; precipitation record semantics; any deployment (systemd
  unit, reverse proxy, TLS) — the app has only been run locally.

## File map

```
main.go                  flags, wiring, graceful shutdown, -reparse
webembed.go              go:embed all:web/build
internal/metar/          tolerant METAR parser + tests
internal/store/          SQLite schema, upsert, Series, records snapshot, Reparse + tests
internal/ingest/         IEM month backfill, AWC live poller, Run loop + tests
internal/api/            /api/summary, /api/series, /api/latest, /api/status, static + SPA + tests
web/                     SvelteKit app (routes, lib/Chart.svelte, lib/Window.svelte, lib/units.js)
web/build/               static build, gitignored, REQUIRED for go:embed
docs/superpowers/        design + implementation plan (the spec of record)
tools/cdp.mjs            CDP helper: prints console errors, exceptions, /api responses for a URL
tools/cdp-click.mjs      CDP helper: navigate, click a preset button, dump text, screenshot
HANDOFF.md               this file
CLAUDE.md                pointer telling a new session to read this file
km40.db (+ -wal/-shm)    live database, untracked, 54 MB
km40                     built binary, untracked, 17 MB
```

`README.md` documents build, run, dev, and flags. The design doc is the source of truth for
scope; the plan file still has unchecked boxes and predates several fixes — trust the code
and this handoff over the plan's task list.

## Gotchas and known problems

- **Svelte 5 legacy mode:** do not call component-local functions from markup to compute
  props or text (`rangeHours={spanHours()}`, `{series()}`). They can read stale state. Use
  `$: span = …` and pass `span`. This cost real time this session; see `web/src/routes/`.
- **`pkill -f "km40 -addr …"` kills the agent's own shell** (exit 143). Use
  `for p in $(pgrep -x km40); do kill $p; done`. Start the server with the background shell
  tool, not `&` inside a normal command.
- **`web/build` is gitignored but embedded.** `go build` fails or embeds stale files until
  `npm --prefix web run build` has run. Order: npm build → go build.
- **The drive is slow:** one 337k-row scan is ~1 s. That is why records are cached and why
  `/api/summary` is concurrent. Adding per-series raw-extreme queries doubled the All-series
  cost from ~1.1 s to ~1.6 s; acceptable, but do not add more full scans casually.
- **Records snapshot staleness:** `RefreshRecords` runs at startup and hourly after live
  ingests; the `cache` row can lag by up to an hour. `-reparse` does not refresh records
  itself — restart after it.
- **Chart bucket floor** can precede the requested `from`; point times are clamped to `from`
  in `Series`, but the bucket still contains only observations inside the window.
- **IEM archive gaps are real:** 2015-04…06 (48 / 0 / 227 reports) and 2016-11…2017-09
  (0 reports). Do not "fix" these by interpolation.
- **Known data oddity:** the 2019-10-26 43 °C reading is what the station reported.
- **The All-time window shows `Record high —` / `Record low —`** on purpose: the window
  covers every archived year, so there is nothing outside it to compare with.
- **Untracked 54 MB DB:** `git clone` gives you code only; a fresh run re-backfills from IEM
  (about three minutes at one request per month).

## How to confirm state on resume

```sh
cd /mnt/ydrive/km40
git status --short                 # expect only handoff files if not yet committed
git log --oneline -3               # expect a5726ac at HEAD and on origin/main
git remote -v                      # origin = https://github.com/robwestbrook/km40.git
go test -count=1 ./...             # expect 4 ok packages
curl -s 127.0.0.1:8099/api/status  # observations 337,190+, last within ~1 h of now
pgrep -x km40                      # empty if the dev server was stopped
```

If `/api/status` is empty, start the server with the background shell tool:
`./km40 -addr 127.0.0.1:8099 -db /mnt/ydrive/km40/km40.db` (records load instantly from the
snapshot). To repeat the browser check:

```sh
~/.cache/ms-playwright/chromium-1254/chrome-linux64/chrome \
  --headless=new --disable-gpu --no-sandbox --remote-debugging-port=9222 about:blank &
node tools/cdp-click.mjs http://127.0.0.1:8099/element/temp All /tmp/all.png
```

`tools/cdp.mjs <url>` prints console errors, exceptions, and `/api/*` responses for a page.

## Context the next session cannot get elsewhere

- The owner is Rob Westbrook, using this as a personal dashboard; he is weather-literate and
  wants US units on screen while storage stays metric.
- Station facts: KM40, Aberdeen / Monroe County Airport, MS; 33.8738 N, 88.4897 W, 68 ft,
  America/Chicago. IEM's station id is **M40** (not KM40); archive starts 2012-01-11.
- Data sources: AWC `https://aviationweather.gov/api/data/metar?ids=KM40&format=raw&hours=6`
  (live) and IEM `https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py` with
  `station=M40&data=metar&…&format=onlycomma` (CSV `station,valid,metar`, split with
  `SplitN(line, ",", 3)` so commas in remarks survive).
- The session ran under the user's "ponytail" minimal-change mode, active until they said
  stop; they never said stop. That is why the min/max band and precip rework were proposed
  rather than built.
- The last user messages were purely git operations ("Init a git repo…", "Run these
  commands…", "Push to origin"), all done; no new product requirements arrived after the
  "Build as you have presented" approval.
