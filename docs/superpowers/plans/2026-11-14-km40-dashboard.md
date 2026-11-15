# KM40 Weather Dashboard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A single Go binary serving a SvelteKit dashboard of every element METAR KM40 reports, with 2012-onward history, window stats, and station records.

**Architecture:** Go (`net/http` only) embeds the static SvelteKit build and serves `/api/*`. SQLite (`modernc.org/sqlite`) stores one row per METAR, keyed by observation time. One tolerant METAR parser serves both the IEM history backfill and the live AWC poller. Charts are drawn client-side from server-bucketed points.

**Tech Stack:** Go 1.22+, `modernc.org/sqlite`, SvelteKit 2, Tailwind CSS 4, Chart.js.

**Spec:** `docs/superpowers/specs/2026-11-14-km40-dashboard-design.md`

## Global Constraints

- Go 1.22+; standard library only for HTTP (no router or web framework).
- Exactly one third-party Go dependency: `modernc.org/sqlite` (non-CGO).
- SvelteKit 2, Tailwind CSS 4, Chart.js. No chart wrapper package, no date adapter.
- Station: KM40, 33.8738 N, 88.4897 W, 68 ft, `America/Chicago`.
- Records are station records since 2012-01-11, per calendar day, excluding the displayed window.
- Store timestamps as UTC unix seconds; render in station local time.
- Never lose a report: raw METAR text is always persisted next to parsed columns.
- Backfill chunks are month-sized, delayed, resumable, and idempotent (upsert).

## Review Focus

- A METAR with no temperature, a missing group, or a malformed token must still be stored (raw text) rather than dropped or crash the poller.
- Day-of-month rollover: a report at 23:55Z on the last day of a month must land on the right date, not "now's month".
- Records must never use the window's own data, so today's high cannot be compared with itself.
- An empty database must serve a working page (no backfill yet) instead of a 500.
- A 14-year range must return a few hundred points, not 350k.

---

### Task 1: METAR parser

**Files:**
- Create: `go.mod`, `internal/metar/metar.go`, `internal/metar/metar_test.go`

**Interfaces:**
- Produces: `metar.Obs` (fields `Time time.Time`, `Station`, `Raw`, `TempC`, `DewpointC`, `WindDir *int`, `WindKt`, `GustKt`, `VisSM`, `Sky`, `CeilingFt *int`, `Wx`, `AltimeterInHg`, `PrecipIn` — numeric fields `*float64`), `metar.Parse(raw string, now time.Time) (Obs, error)`, `metar.RelativeHumidity(tempC, dewC float64) float64`.

- [ ] **Step 1: Write failing tests** for: the live example `KM40 150115Z AUTO 00000KT 10SM CLR 04/00 A3028 RMK AO2` (4 °C, calm, 10 SM, altimeter 30.28, CLR); gust `21015G25KT`; variable `VRB03KT`; missing dew point `04/`; negative `M04/M07`; fraction visibility `1/2SM` and mixed `1 1/2SM`; weather `-RA BR`; ceiling from `BKN008` under `SCT020`; `Q1013` altimeter ≈ 29.91; `P0013` precip 0.13; month rollover (report `010005Z` with `now` = Jan 2).
- [ ] **Step 2: Run tests, expect failure** — `go test ./internal/metar`.
- [ ] **Step 3: Implement** the token scanner above; unknown groups are ignored, a missing timestamp is the only error.
- [ ] **Step 4: Run tests, expect pass.**
- [ ] **Step 5: Commit.**

### Task 2: SQLite store

**Files:**
- Create: `internal/store/store.go`, `internal/store/store_test.go`

**Interfaces:**
- Consumes: `metar.Obs`, `metar.RelativeHumidity`.
- Produces: `store.Open(path string) (*Store, error)`, `(*Store).Upsert(ctx, []metar.Obs) (int, error)`, `(*Store).Count(ctx) (int, error)`, `(*Store).Latest(ctx) (*metar.Obs, error)`, `(*Store).MonthDone(ctx, ym string) (bool, error)`, `(*Store).MarkMonth(ctx, ym string) error`, `(*Store).RefreshRecords(ctx) error`, `(*Store).Series(ctx, element string, from, to time.Time, points int) (Series, error)`, `(*Store).Elements() []Element` where `Element{Key, Label, Unit string; Precision int}`, `Series{Points []Point; Min, Max, Avg float64; MinAt, MaxAt time.Time; RecordHigh, RecordLow float64; RecordHighDate, RecordLowDate string}` and `Point{T time.Time; V float64}`.
- Element keys: `temp`, `dewpoint`, `humidity`, `wind`, `gust`, `visibility`, `ceiling`, `altimeter`, `precip`.

- [ ] **Step 1: Write failing tests** against `t.TempDir()` DBs: upserting twice inserts once; `Latest` returns the newest; `Series` over 48 h with `points=24` returns ≤ 25 buckets; records exclude the queried window; humidity is computed from temp and dew point.
- [ ] **Step 2: Run tests, expect failure.**
- [ ] **Step 3: Implement** schema (`observations(obs_time INTEGER PRIMARY KEY, raw TEXT NOT NULL, temp_c, dewpoint_c, rh, wind_dir, wind_kt, gust_kt, vis_sm, sky TEXT, ceiling_ft, wx TEXT, altimeter_inhg, precip_in)`), `INSERT … ON CONFLICT(obs_time) DO UPDATE`, bucketed `Series` via `obs_time / bucket * bucket`, and in-memory per-MM-DD records refreshed by `RefreshRecords`.
- [ ] **Step 4: Run tests, expect pass.**
- [ ] **Step 5: Commit.**

### Task 3: Ingest (backfill + poller)

**Files:**
- Create: `internal/ingest/ingest.go`, `internal/ingest/ingest_test.go`

**Interfaces:**
- Consumes: `*store.Store`, `metar.Parse`.
- Produces: `ingest.Live(ctx, st, client *http.Client) (int, error)`, `ingest.Backfill(ctx, st, client *http.Client, from, to time.Time, log func(string)) error`, `ingest.Run(ctx, st, client *http.Client, log func(string)) error` (poller every 5 min, backfill first).

- [ ] **Step 1: Write failing tests** with `httptest` servers: Live parses two raw METARs and upserts both; a garbage line is skipped without failing the batch; Backfill skips months marked done.
- [ ] **Step 2: Run tests, expect failure.**
- [ ] **Step 3: Implement** AWC raw feed (`/api/data/metar?ids=KM40&format=raw&hours=6`) and IEM month requests (`/cgi-bin/request/asos.py?station=M40&data=metar&…&format=onlycomma`), 1 s delay between months, `User-Agent` set, 30 s timeout.
- [ ] **Step 4: Run tests, expect pass.**
- [ ] **Step 5: Commit.**

### Task 4: HTTP API and static serving

**Files:**
- Create: `internal/api/api.go`, `internal/api/api_test.go`, `main.go`, `webembed.go`

**Interfaces:**
- Consumes: `*store.Store`.
- Produces: `api.New(st *store.Store, static fs.FS) http.Handler`; routes `GET /api/summary`, `GET /api/series`, `GET /api/status`; unknown non-file paths serve `200.html`.

- [ ] **Step 1: Write failing tests**: `/api/summary` returns 200 with 9 elements and a `sentence` on an empty DB; `/api/series?element=temp&points=10` returns 400 for a bad element; `/api/status` returns counts.
- [ ] **Step 2: Run tests, expect failure.**
- [ ] **Step 3: Implement** handlers (`from`/`to` accept `2006-01-02` or RFC3339; defaults to the last 24 h), the decoded summary sentence, and the static handler with SPA fallback.
- [ ] **Step 4: Run tests, expect pass.**
- [ ] **Step 5: Commit.**

### Task 5: SvelteKit UI

**Files:**
- Create: `web/` (SvelteKit scaffold), `web/src/routes/+layout.ts`, `web/src/routes/+page.svelte`, `web/src/routes/element/[key]/+page.svelte`, `web/src/lib/Chart.svelte`, `web/src/lib/api.ts`, `web/src/app.css`, `web/vite.config.ts`.

**Interfaces:**
- Consumes: `/api/summary`, `/api/series`, `/api/status`.

- [ ] **Step 1:** Scaffold SvelteKit (`adapter-static`, fallback `200.html`, `ssr = false`), Tailwind 4 via `@tailwindcss/vite`, Chart.js, `/api` dev proxy to `:8080`.
- [ ] **Step 2:** Dashboard: centered summary line, 9 section cards (current, window high/low, record high/low with dates, sparkline, "Details →").
- [ ] **Step 3:** Element page: large chart, 24 h/7 d/30 d/1 y/All presets plus from/to date inputs, stats row.
- [ ] **Step 4:** `npm --prefix web run build` succeeds.
- [ ] **Step 5: Commit.**

### Task 6: End-to-end verification

- [ ] **Step 1:** Build the binary with the real backfill; confirm `/api/status` reports observations from 2012 and the newest within the last hour.
- [ ] **Step 2:** Confirm `/api/summary` shows 9 elements with record values; confirm a 14-year series returns ≤ 400 points.
- [ ] **Step 3:** Fetch `/` and `/element/temp` and confirm HTML is served (SPA fallback).
- [ ] **Step 4:** Commit and report actual numbers.
