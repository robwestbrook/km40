# KM40 weather dashboard

A single-binary dashboard for METAR station **KM40** — Aberdeen / Monroe County
Airport, Mississippi. Every element the station reports gets a card with the
current value, the window's high and low, the station's records for those
calendar days, and a graph; each card links to a full page with an arbitrary
date range.

- Backend: Go, `net/http` only, SQLite via `modernc.org/sqlite` (no CGO).
- Frontend: SvelteKit 2 + Tailwind CSS 4 + Chart.js, built to static files and
  embedded in the Go binary.
- History: 2012-01-11 onward, backfilled once from the IEM archive, then kept
  current from the Aviation Weather Center feed every five minutes.
- Records are **station records since 2012**, computed per calendar day and
  always excluding the window being displayed.

## Run

```sh
npm --prefix web install
npm --prefix web run build      # writes web/build, embedded by go:embed
go build -o km40 .
./km40                          # http://localhost:8080
```

First start backfills ~14 years of observations (a few minutes, one request per
month, resumable — completed months are recorded in the database).

Flags: `-addr :8080`, `-db km40.db`, `-no-ingest`, `-reparse`.

## Develop

```sh
./km40 -addr :8080              # API on :8080
npm --prefix web run dev        # UI on :5173, proxies /api to :8080
```

## Check

```sh
go test ./...
curl -s localhost:8080/api/status
```

## Layout

```
main.go                 flags, wiring, graceful shutdown
webembed.go             go:embed of web/build
internal/metar/         tolerant METAR parser (shared by backfill and poller)
internal/store/         SQLite schema, upserts, bucketed series, records
internal/ingest/        IEM month backfill + AWC live poller
internal/api/           JSON endpoints and static serving with SPA fallback
web/                    SvelteKit app
docs/superpowers/       design and plan
```

## Notes

- `-reparse` re-decodes every stored report from the saved raw METAR text, so
  parser fixes apply to history without re-downloading.
- Values are stored in canonical units (°C, knots, miles, inHg, feet) and
  converted for display to °F, mph, mi, inHg, ft.
