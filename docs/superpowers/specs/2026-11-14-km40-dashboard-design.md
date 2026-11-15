# KM40 weather dashboard — design

## Intent

A personal dashboard for METAR station **KM40** (Aberdeen/Monroe County Airport,
Mississippi; 33.8738 N, 88.4897 W, 68 ft, America/Chicago).

One screen answers "what is it doing now, and how does that compare to normal
for this date" for every element the station reports. Each element also gets
its own page for full-size history.

## Goals

- Centered summary line at the top: latest observation decoded into one plain
  sentence, with the observation time.
- One section per reported element: temperature, dew point, relative humidity,
  wind (speed, gust, direction), visibility, ceiling, sky cover, altimeter,
  precipitation.
- Each section: current value, window high, window low, record high and record
  low for the same calendar days, a small graph, and a link to that element's
  page.
- Element pages: large graph, default 24 h window, selectable to any date range
  (presets plus explicit from/to dates), window stats and records with dates.
- History works on first run: backfilled from IEM's archive (starts
  2012-01-11), then kept current by polling the Aviation Weather Center.

## Non-goals

- Forecasts, radar, satellite, alerts, webcams.
- Multi-station support, user accounts, sharing, mobile apps.
- Official climate products. Records are **station records since 2012**, not
  NOAA climate records.

## Anti-goals

- Anything that needs a second process, a queue, or a container in production.
- Charts that require downloading years of raw observations into the browser.
- Data loss on parse failure: raw METAR text is always stored.

## Constraints

- Go 1.22+ standard library for the web server (no router/framework).
- SvelteKit 2, Tailwind CSS 4.
- SQLite via a non-CGO driver (`modernc.org/sqlite`).
- Public data sources only, no API keys. IEM rate-limits; backfill is chunked
  and resumable.

## Approaches considered

**Shipping.** (a) Embed SvelteKit's static build in the Go binary with
`go:embed`, serve it and `/api/*` from one process. (b) Run SvelteKit's Node
server and reverse-proxy Go behind it. Chosen: **a** — one binary, same-origin,
no proxy or CORS in production; Vite proxies `/api` in dev.

**History.** (a) One-time IEM backfill plus live AWC polling. (b) Live
collection only, starting empty. (c) Keep polling IEM for new data. Chosen:
**a** — graphs and records are useful immediately; AWC is the cheap,
unauthenticated live feed.

**Charts.** (a) Server-side bucketing plus Chart.js. (b) Raw points to the
browser with client-side decimation. Chosen: **a** — a 14-year range becomes
~400 points, and no date-adapter dependency is needed (labels are preformatted).

## Decisions

- Storage: UTC unix seconds; display in station local time. `observations`
  table keyed by observation time, one row per METAR, raw text alongside
  parsed columns. Upsert on `obs_time` makes every ingest path idempotent.
- Parsing: one tolerant METAR parser shared by backfill and live polling.
  Unknown or unparseable groups are skipped, never fatal; the raw report is
  kept so future parser fixes can replay history.
- Records: per calendar day (MM-DD), all-time max and min since 2012 for each
  element, **excluding the window being displayed** so a day is never compared
  against itself.
- API: `/api/summary`, `/api/series?element=&from=&to=&points=`,
  `/api/latest` for the current observation only, and `/api/status` for
  backfill/poller health. Chart points are bucket averages; the window's high
  and low are taken from the raw observations, with the time they occurred.
- Frontend: SPA fallback (`200.html`) so element routes work without knowing
  every route at build time.
- Backfill: month-sized IEM requests with a delay, skipping months already
  present, so an interrupted backfill resumes on restart.
- Polling: AWC raw METARs for the last few hours every 5 minutes; upsert makes
  overlaps free.

## Left to the builder

Exact chart styling, section ordering, responsive layout, and the precise
wording of the summary sentence.
