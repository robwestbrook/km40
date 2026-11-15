package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"km40/internal/metar"
)

// ErrUnknownElement is returned for an element key the store does not have; it
// is a caller mistake, unlike a database failure.
var ErrUnknownElement = errors.New("store: unknown element")

// Element is one reported data point, in stored (canonical) units.
type Element struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	Unit      string `json:"unit"`
	Precision int    `json:"precision"`
	column    string
	agg       string
}

var elements = []Element{
	{Key: "temp", Label: "Temperature", Unit: "°C", Precision: 1, column: "temp_c", agg: "avg"},
	{Key: "dewpoint", Label: "Dew point", Unit: "°C", Precision: 1, column: "dewpoint_c", agg: "avg"},
	{Key: "humidity", Label: "Relative humidity", Unit: "%", Precision: 0, column: "rh", agg: "avg"},
	{Key: "wind", Label: "Wind speed", Unit: "kt", Precision: 0, column: "wind_kt", agg: "avg"},
	{Key: "gust", Label: "Wind gust", Unit: "kt", Precision: 0, column: "gust_kt", agg: "max"},
	{Key: "visibility", Label: "Visibility", Unit: "mi", Precision: 1, column: "vis_sm", agg: "min"},
	{Key: "ceiling", Label: "Ceiling", Unit: "ft", Precision: 0, column: "ceiling_ft", agg: "min"},
	{Key: "sky", Label: "Sky cover", Unit: "%", Precision: 0, column: "sky_cover", agg: "avg"},
	{Key: "altimeter", Label: "Altimeter", Unit: "inHg", Precision: 2, column: "altimeter_inhg", agg: "avg"},
	{Key: "precip", Label: "Precipitation", Unit: "in", Precision: 2, column: "precip_in", agg: "sum"},
}

// Elements returns the supported elements in display order.
func Elements() []Element {
	out := make([]Element, len(elements))
	copy(out, elements)
	return out
}

func element(key string) (Element, bool) {
	for _, e := range elements {
		if e.Key == key {
			return e, true
		}
	}
	return Element{}, false
}

type Point struct {
	T time.Time `json:"t"`
	V float64   `json:"v"`
}

type Series struct {
	Element        string    `json:"element"`
	Unit           string    `json:"unit"`
	Precision      int       `json:"precision"`
	Aggregate      string    `json:"aggregate"`
	Points         []Point   `json:"points"`
	HasData        bool      `json:"hasData"`
	Min            float64   `json:"min"`
	Max            float64   `json:"max"`
	Avg            float64   `json:"avg"`
	MinAt          time.Time `json:"minAt"`
	MaxAt          time.Time `json:"maxAt"`
	RecordHigh     float64   `json:"recordHigh"`
	RecordLow      float64   `json:"recordLow"`
	HasRecord      bool      `json:"hasRecord"`
	RecordHighDate string    `json:"recordHighDate"`
	RecordLowDate  string    `json:"recordLowDate"`
}

// daily is one calendar day's extreme, with the observation that produced it.
// Fields are exported so the records snapshot can be cached as JSON.
type daily struct {
	High   float64   `json:"h"`
	Low    float64   `json:"l"`
	AtHigh time.Time `json:"ah"`
	AtLow  time.Time `json:"al"`
}

// extreme is one MAX or MIN for a calendar date, with the observation that set it.
type extreme struct {
	v  float64
	at time.Time
}

type Store struct {
	db *sql.DB

	refreshMu sync.Mutex // serialises RefreshRecords

	mu      sync.RWMutex
	records map[string]map[string]map[string]daily // element -> "01-02" -> "2006-01-02" -> day
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4) // WAL: readers do not block the single writer
	for _, q := range []string{
		`PRAGMA journal_mode=WAL`,
		`PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS observations (
			obs_time INTEGER PRIMARY KEY,
			raw TEXT NOT NULL,
			temp_c REAL, dewpoint_c REAL, rh REAL,
			wind_dir INTEGER, wind_kt REAL, gust_kt REAL,
			vis_sm REAL, sky TEXT, ceiling_ft INTEGER, wx TEXT,
			altimeter_inhg REAL, precip_in REAL, sky_cover REAL)`,
		`CREATE TABLE IF NOT EXISTS backfill (ym TEXT PRIMARY KEY)`,
		`CREATE TABLE IF NOT EXISTS cache (key TEXT PRIMARY KEY, value BLOB NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS observations_cols ON observations (obs_time)`,
		// Months marked before the IEM end-date fix are missing their last day.
		// Clear the marks once so the next backfill refetches them.
		`DELETE FROM backfill WHERE NOT EXISTS (SELECT 1 FROM cache WHERE key = 'backfill_end_exclusive')`,
		`INSERT OR IGNORE INTO cache (key, value) VALUES ('backfill_end_exclusive', x'01')`,
		// The records snapshot used to be keyed by UTC date; records are now
		// per station-local day. Drop the stale snapshot once so it rebuilds.
		`DELETE FROM cache WHERE key = 'records' AND NOT EXISTS (SELECT 1 FROM cache WHERE key = 'records_local_day')`,
		`INSERT OR IGNORE INTO cache (key, value) VALUES ('records_local_day', x'01')`,
	} {
		if _, err := db.Exec(q); err != nil {
			db.Close()
			return nil, fmt.Errorf("store: %s: %w", q, err)
		}
	}
	if err := migrateSkyCover(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db, records: map[string]map[string]map[string]daily{}}, nil
}

// migrateSkyCover adds the sky_cover column to a database created before it
// existed and fills it from the stored sky text, once. The fill reuses the same
// expression as the Go function so the two cannot disagree. A stored column
// keeps sky cover as cheap to aggregate as every other element; computing it
// per row at query time made a 14-year window several times slower.
func migrateSkyCover(db *sql.DB) error {
	var has int
	if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('observations') WHERE name = 'sky_cover'`).Scan(&has); err != nil {
		return fmt.Errorf("store: inspect observations: %w", err)
	}
	if has == 0 {
		if _, err := db.Exec(`ALTER TABLE observations ADD COLUMN sky_cover REAL`); err != nil {
			return fmt.Errorf("store: add sky_cover: %w", err)
		}
	}
	var filled int
	if err := db.QueryRow(`SELECT COUNT(*) FROM cache WHERE key = 'sky_cover_filled'`).Scan(&filled); err != nil {
		return err
	}
	if filled > 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`UPDATE observations SET sky_cover = ` + metar.SkyCoverSQL("sky")); err != nil {
		return fmt.Errorf("store: fill sky_cover: %w", err)
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO cache (key, value) VALUES ('sky_cover_filled', x'01')`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) Upsert(ctx context.Context, obs []metar.Obs) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	stmt, err := tx.PrepareContext(ctx, `INSERT INTO observations
		(obs_time, raw, temp_c, dewpoint_c, rh, wind_dir, wind_kt, gust_kt,
		 vis_sm, sky, ceiling_ft, wx, altimeter_inhg, precip_in, sky_cover)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(obs_time) DO UPDATE SET
			raw=excluded.raw, temp_c=excluded.temp_c, dewpoint_c=excluded.dewpoint_c,
			rh=excluded.rh, wind_dir=excluded.wind_dir, wind_kt=excluded.wind_kt,
			gust_kt=excluded.gust_kt, vis_sm=excluded.vis_sm, sky=excluded.sky,
			ceiling_ft=excluded.ceiling_ft, wx=excluded.wx,
			altimeter_inhg=excluded.altimeter_inhg, precip_in=excluded.precip_in,
			sky_cover=excluded.sky_cover`)
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	n := 0
	for _, o := range obs {
		if o.Time.IsZero() {
			continue
		}
		var rh *float64
		if o.TempC != nil && o.DewpointC != nil {
			v := metar.RelativeHumidity(*o.TempC, *o.DewpointC)
			rh = &v
		}
		var skyCover *float64
		if pct, ok := metar.SkyCoverPercent(o.Sky); ok {
			skyCover = &pct
		}
		if _, err := stmt.ExecContext(ctx,
			o.Time.Unix(), o.Raw, o.TempC, o.DewpointC, rh,
			o.WindDir, o.WindKt, o.GustKt, o.VisSM, o.Sky, o.CeilingFt,
			o.Wx, o.AltimeterInHg, o.PrecipIn, skyCover); err != nil {
			return n, err
		}
		n++
	}
	return n, tx.Commit()
}

// Reparse re-decodes every stored report from its raw text, so a parser fix
// can be applied to history. Rows are read into memory first so the read
// cursor is closed before the write transaction starts.
func (s *Store) Reparse(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT obs_time, raw FROM observations`)
	if err != nil {
		return 0, err
	}
	var obs []metar.Obs
	for rows.Next() {
		var ts int64
		var raw string
		if err := rows.Scan(&ts, &raw); err != nil {
			rows.Close()
			return 0, err
		}
		o, err := metar.Parse(raw, time.Unix(ts, 0).UTC())
		if err != nil {
			continue
		}
		o.Time = time.Unix(ts, 0).UTC()
		obs = append(obs, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	return s.Upsert(ctx, obs)
}

func (s *Store) Count(ctx context.Context) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM observations`).Scan(&n)
	return n, err
}

// Span reports the oldest and newest observation times, zero when empty.
func (s *Store) Span(ctx context.Context) (first, last time.Time, err error) {
	var lo, hi sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(obs_time), MAX(obs_time) FROM observations`).Scan(&lo, &hi); err != nil {
		return time.Time{}, time.Time{}, err
	}
	if lo.Valid {
		first = time.Unix(lo.Int64, 0).UTC()
	}
	if hi.Valid {
		last = time.Unix(hi.Int64, 0).UTC()
	}
	return first, last, nil
}

func (s *Store) Latest(ctx context.Context) (*metar.Obs, error) {
	var raw string
	var ts int64
	err := s.db.QueryRowContext(ctx,
		`SELECT raw, obs_time FROM observations ORDER BY obs_time DESC LIMIT 1`).Scan(&raw, &ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	o, err := metar.Parse(raw, time.Unix(ts, 0).UTC())
	if err != nil {
		// An undecodable report is still the latest observation. Parse never
		// fails hard on stored data; keep the raw text and the stored time.
		o.Time = time.Unix(ts, 0).UTC()
	}
	return &o, nil
}

func (s *Store) MonthDone(ctx context.Context, ym string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM backfill WHERE ym = ?`, ym).Scan(&n)
	return n > 0, err
}

func (s *Store) MarkMonth(ctx context.Context, ym string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO backfill (ym) VALUES (?)`, ym)
	return err
}

// RefreshRecords rebuilds the per-calendar-day records in memory: for every
// MM-DD, one entry per year that has data.
func (s *Store) RefreshRecords(ctx context.Context) error {
	// A rebuild is a full-table scan (about 30 s on this drive). Startup and
	// the post-backfill refresh can both ask for one; run them one at a time.
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()
	recs := make(map[string]map[string]map[string]daily, len(elements))
	for _, e := range elements {
		highs, err := s.dailyExtreme(ctx, e.column, "MAX")
		if err != nil {
			return err
		}
		lows, err := s.dailyExtreme(ctx, e.column, "MIN")
		if err != nil {
			return err
		}
		m := map[string]map[string]daily{}
		for date, h := range highs {
			day := date[5:]
			if m[day] == nil {
				m[day] = map[string]daily{}
			}
			d := m[day][date]
			d.High, d.AtHigh = h.v, h.at
			m[day][date] = d
		}
		for date, l := range lows {
			day := date[5:]
			if m[day] == nil {
				m[day] = map[string]daily{}
			}
			d := m[day][date]
			d.Low, d.AtLow = l.v, l.at
			m[day][date] = d
		}
		recs[e.Key] = m
	}
	s.mu.Lock()
	s.records = recs
	s.mu.Unlock()
	// The in-memory records are already live; a snapshot that cannot be saved
	// only costs the next restart a rebuild, but the caller should know.
	blob, err := json.Marshal(recs)
	if err != nil {
		return fmt.Errorf("records snapshot: %w", err)
	}
	if _, err := s.db.ExecContext(ctx, `INSERT OR REPLACE INTO cache (key, value) VALUES ('records', ?)`, blob); err != nil {
		return fmt.Errorf("records snapshot: %w", err)
	}
	return nil
}

// LoadRecords restores the cached records snapshot, so a restart can serve
// records immediately while RefreshRecords runs in the background.
func (s *Store) LoadRecords(ctx context.Context) error {
	var blob []byte
	err := s.db.QueryRowContext(ctx, `SELECT value FROM cache WHERE key = 'records'`).Scan(&blob)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	var recs map[string]map[string]map[string]daily
	if err := json.Unmarshal(blob, &recs); err != nil {
		return err
	}
	s.mu.Lock()
	s.records = recs
	s.mu.Unlock()
	return nil
}

// dailyExtreme returns the MAX or MIN per station-local calendar date
// ("2006-01-02"). SQLite takes bare columns from the extreme row when the only
// aggregate is min/max.
func (s *Store) dailyExtreme(ctx context.Context, column, fn string) (map[string]extreme, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT local_day(obs_time) d, %s(%s) v, obs_time
		 FROM observations WHERE %s IS NOT NULL GROUP BY d`, fn, column, column))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]extreme{}
	for rows.Next() {
		var date string
		var v float64
		var ts int64
		if err := rows.Scan(&date, &v, &ts); err != nil {
			return nil, err
		}
		out[date] = extreme{v: v, at: time.Unix(ts, 0).UTC()}
	}
	return out, rows.Err()
}

// Series returns bucketed values for the window, plus window stats and the
// record high/low for the same calendar days taken from outside the window.
func (s *Store) Series(ctx context.Context, key string, from, to time.Time, points int) (Series, error) {
	e, ok := element(key)
	if !ok {
		return Series{}, fmt.Errorf("%w %q", ErrUnknownElement, key)
	}
	if points < 1 {
		points = 240
	}
	if !to.After(from) {
		to = from.Add(24 * time.Hour)
	}
	seconds := int64(to.Sub(from).Seconds())
	bucket := seconds / int64(points)
	if bucket < 60 {
		bucket = 60
	}

	agg := e.agg
	if agg == "min" {
		agg = "MIN"
	} else if agg == "max" {
		agg = "MAX"
	} else if agg == "sum" {
		agg = "SUM"
	} else {
		agg = "AVG"
	}
	q := fmt.Sprintf(`SELECT (obs_time/?)*? b, %s(%s) v, COUNT(%s)
		FROM observations
		WHERE obs_time BETWEEN ? AND ? AND %s IS NOT NULL
		GROUP BY b ORDER BY b`, agg, e.column, e.column, e.column)
	rows, err := s.db.QueryContext(ctx, q, bucket, bucket, from.Unix(), to.Unix())
	if err != nil {
		return Series{}, err
	}
	defer rows.Close()

	out := Series{Element: e.Key, Unit: e.Unit, Precision: e.Precision, Aggregate: e.agg, Points: []Point{}}
	var sum float64
	for rows.Next() {
		var b, n int64
		var v float64
		if err := rows.Scan(&b, &v, &n); err != nil {
			return Series{}, err
		}
		t := time.Unix(b, 0).UTC()
		if t.Before(from) {
			t = from // bucket floor can precede the requested start
		}
		p := Point{T: t, V: v}
		out.Points = append(out.Points, p)
		out.HasData = true
		if len(out.Points) == 1 || v < out.Min {
			out.Min, out.MinAt = v, p.T
		}
		if len(out.Points) == 1 || v > out.Max {
			out.Max, out.MaxAt = v, p.T
		}
		sum += v
	}
	if err := rows.Err(); err != nil {
		return Series{}, err
	}
	if len(out.Points) > 0 {
		out.Avg = sum / float64(len(out.Points))
	}
	if agg == "SUM" {
		// Summed points are per-bucket totals; the window total is their sum.
		out.Avg = sum
	}

	// Bucketed values are averages, so their extremes understate a long
	// window. Take the true high and low from the raw observations.
	if agg != "SUM" {
		for _, x := range []struct {
			fn string
			v  *float64
			at *time.Time
		}{
			{"MIN", &out.Min, &out.MinAt},
			{"MAX", &out.Max, &out.MaxAt},
		} {
			var v sql.NullFloat64
			var ts sql.NullInt64
			q := fmt.Sprintf(`SELECT %s(%s), obs_time FROM observations
				WHERE obs_time BETWEEN ? AND ? AND %s IS NOT NULL`, x.fn, e.column, e.column)
			if err := s.db.QueryRowContext(ctx, q, from.Unix(), to.Unix()).Scan(&v, &ts); err != nil {
				return Series{}, err
			}
			if v.Valid && ts.Valid {
				*x.v, *x.at = v.Float64, time.Unix(ts.Int64, 0).UTC()
			}
		}
	}

	high, low, highDate, lowDate, ok := s.recordsFor(e.Key, from, to)
	if ok {
		out.HasRecord = true
		out.RecordHigh, out.RecordLow = high, low
		out.RecordHighDate, out.RecordLowDate = highDate, lowDate
	}
	return out, nil
}

// recordsFor takes the record high and low across the station-local calendar
// days covered by the window, using only observations from outside the window
// so a day is never compared against itself. It returns the local dates
// ("2006-01-02") the records were set on.
func (s *Store) recordsFor(key string, from, to time.Time) (high, low float64, highDate, lowDate string, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	days := s.records[key]
	if len(days) == 0 {
		return 0, 0, "", "", false
	}
	firstDay, lastDay := LocalDay(from), LocalDay(to)
	// Walk calendar days from noon, so adding a day never lands on a daylight
	// saving gap or repeat; the zone's 23 h and 25 h days still count once.
	f := from.In(Local)
	day := time.Date(f.Year(), f.Month(), f.Day(), 12, 0, 0, 0, Local)
	seen := map[string]bool{}
	for ; day.Format("2006-01-02") <= lastDay; day = day.AddDate(0, 0, 1) {
		mmdd := day.Format("01-02")
		if seen[mmdd] {
			continue
		}
		seen[mmdd] = true
		for date, d := range days[mmdd] {
			if date >= firstDay && date <= lastDay {
				continue // the window's own days
			}
			// Ties go to the earliest date, so the result does not depend on
			// map iteration order.
			if !ok || d.High > high || (d.High == high && date < highDate) {
				high, highDate = d.High, date
			}
			if !ok || d.Low < low || (d.Low == low && date < lowDate) {
				low, lowDate = d.Low, date
			}
			ok = true
		}
		if len(seen) >= 366 {
			break
		}
	}
	return high, low, highDate, lowDate, ok
}

// ElementKeys is a convenience for validation messages.
func ElementKeys() string {
	keys := make([]string, 0, len(elements))
	for _, e := range elements {
		keys = append(keys, e.Key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}
