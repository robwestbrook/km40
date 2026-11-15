package store

import (
	"context"
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"km40/internal/metar"
)

func f(v float64) *float64 { return &v }

func chi(t *testing.T, y int, m time.Month, d, h, min int) time.Time {
	t.Helper()
	loc, err := time.LoadLocation("America/Chicago")
	if err != nil {
		t.Fatal(err)
	}
	return time.Date(y, m, d, h, min, 0, 0, loc)
}

// Records are per station-local calendar day. A 22:30 CDT report is already
// the next day in UTC but still belongs to the local day it was taken on.
func TestRecordsUseLocalCalendarDays(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	if _, err := s.Upsert(ctx, []metar.Obs{
		obsAt(chi(t, 2019, 7, 1, 22, 30), 38), // local Jul 1 (UTC Jul 2)
		obsAt(chi(t, 2019, 6, 30, 23, 30), 50), // local Jun 30 (UTC Jul 1): must not leak into Jul 1
		obsAt(chi(t, 2021, 7, 1, 22, 30), 99),  // inside the displayed window: excluded
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshRecords(ctx); err != nil {
		t.Fatal(err)
	}
	from := chi(t, 2021, 7, 1, 0, 0).UTC()
	to := chi(t, 2021, 7, 1, 23, 59).Add(59 * time.Second).UTC()
	ser, err := s.Series(ctx, "temp", from, to, 100)
	if err != nil {
		t.Fatal(err)
	}
	if !ser.HasRecord || ser.RecordHigh != 38 || ser.RecordLow != 38 {
		t.Errorf("record = %v/%v (has=%v), want 38/38", ser.RecordHigh, ser.RecordLow, ser.HasRecord)
	}
	if ser.RecordHighDate != "2019-07-01" {
		t.Errorf("record date = %q, want the local day 2019-07-01", ser.RecordHighDate)
	}
}

// The record lookup must visit every local calendar day the window touches,
// including across the 23 h and 25 h daylight-saving days, and no others.
func TestRecordsCoverEveryLocalDayOfWindow(t *testing.T) {
	cases := []struct {
		name         string
		from, to     [4]int // month, day, hour, minute in 2020
		wantHigh     float64
		wantHighDate string
		wantLow      float64
	}{
		{"spring forward", [4]int{3, 7, 18, 0}, [4]int{3, 9, 17, 0}, 30, "2019-03-07", 10},
		{"fall back", [4]int{10, 31, 20, 0}, [4]int{11, 2, 1, 0}, 25, "2019-11-01", 5},
	}
	days := []struct {
		m    time.Month
		d    int
		temp float64
	}{
		{3, 7, 30}, {3, 8, 20}, {3, 9, 10}, {3, 10, 40}, // Mar 10 is outside the spring window
		{10, 31, 5}, {11, 1, 25}, {11, 2, 15}, {11, 3, 99}, // Nov 3 is outside the fall window
	}
	s := open(t)
	ctx := context.Background()
	for _, d := range days {
		if _, err := s.Upsert(ctx, []metar.Obs{obsAt(chi(t, 2019, d.m, d.d, 12, 0), d.temp)}); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.RefreshRecords(ctx); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// The API hands the store UTC instants, so do the same here.
			from := chi(t, 2020, time.Month(c.from[0]), c.from[1], c.from[2], c.from[3]).UTC()
			to := chi(t, 2020, time.Month(c.to[0]), c.to[1], c.to[2], c.to[3]).UTC()
			ser, err := s.Series(ctx, "temp", from, to, 100)
			if err != nil {
				t.Fatal(err)
			}
			if !ser.HasRecord || ser.RecordHigh != c.wantHigh || ser.RecordLow != c.wantLow {
				t.Errorf("record = %v/%v (has=%v), want %v/%v", ser.RecordHigh, ser.RecordLow, ser.HasRecord, c.wantHigh, c.wantLow)
			}
			if ser.RecordHighDate != c.wantHighDate {
				t.Errorf("high date = %q, want %q", ser.RecordHighDate, c.wantHighDate)
			}
		})
	}
}

// Records computed by the UTC-day code are keyed wrongly; the snapshot is
// dropped once so it rebuilds under the local-day rule.
func TestOpenDropsStaleRecordsSnapshotOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "r.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO cache (key, value) VALUES ('records', x'7b7d')`); err != nil {
		t.Fatal(err)
	}
	// Simulate a database from before the change: snapshot present, no flag.
	if _, err := s.db.Exec(`DELETE FROM cache WHERE key = 'records_local_day'`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	count := func() int {
		s, err := Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		var n int
		if err := s.db.QueryRow(`SELECT COUNT(*) FROM cache WHERE key = 'records'`).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := count(); n != 0 {
		t.Fatalf("stale snapshot survived the first open (rows=%d)", n)
	}
	s, _ = Open(path)
	if _, err := s.db.Exec(`INSERT OR REPLACE INTO cache (key, value) VALUES ('records', x'7b7d')`); err != nil {
		t.Fatal(err)
	}
	s.Close()
	if n := count(); n != 1 {
		t.Errorf("a snapshot written after the change must survive a reopen (rows=%d)", n)
	}
}

// An undecodable newest report is still the latest observation: Latest must
// not fail, or /api/summary goes down until a decodable report arrives.
func TestLatestSurvivesUnparseableReport(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	at := time.Date(2020, 5, 1, 12, 0, 0, 0, time.UTC)
	if _, err := s.Upsert(ctx, []metar.Obs{{Time: at, Raw: "KM40 AUTO 10SM CLR"}}); err != nil {
		t.Fatal(err)
	}
	o, err := s.Latest(ctx)
	if err != nil {
		t.Fatalf("Latest: %v", err)
	}
	if o == nil || !o.Time.Equal(at) || o.Raw != "KM40 AUTO 10SM CLR" {
		t.Errorf("Latest = %+v, want the stored report at %v", o, at)
	}
}

func obsAt(t time.Time, temp float64) metar.Obs {
	return metar.Obs{Time: t, Raw: "KM40 TEST", TempC: f(temp), DewpointC: f(temp - 5)}
}

func open(t *testing.T) *Store {
	t.Helper()
	s, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestUpsertIsIdempotent(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	when := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		if _, err := s.Upsert(ctx, []metar.Obs{obsAt(when, 10)}); err != nil {
			t.Fatal(err)
		}
	}
	if n, _ := s.Count(ctx); n != 1 {
		t.Fatalf("count = %d, want 1", n)
	}
}

func TestUpsertStoresHumidityAndLatest(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	old := metar.Obs{Time: time.Date(2026, 11, 14, 10, 0, 0, 0, time.UTC), Raw: "KM40 141000Z 10SM CLR 20/20 A3010", TempC: f(20), DewpointC: f(20)}
	newer := metar.Obs{Time: time.Date(2026, 11, 14, 11, 0, 0, 0, time.UTC), Raw: "KM40 141100Z 10SM CLR 22/12 A3010", TempC: f(22), DewpointC: f(12)}
	if _, err := s.Upsert(ctx, []metar.Obs{old, newer}); err != nil {
		t.Fatal(err)
	}
	var rh float64
	if err := s.db.QueryRow(`SELECT rh FROM observations WHERE obs_time = ?`, old.Time.Unix()).Scan(&rh); err != nil {
		t.Fatal(err)
	}
	if math.Abs(rh-100) > 0.5 {
		t.Errorf("rh = %v, want 100", rh)
	}
	latest, err := s.Latest(ctx)
	if err != nil || latest == nil {
		t.Fatalf("latest: %v", err)
	}
	if *latest.TempC != 22 {
		t.Errorf("latest temp = %v, want 22", *latest.TempC)
	}
}

func TestSeriesBuckets(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	start := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	var batch []metar.Obs
	for i := 0; i < 96; i++ {
		batch = append(batch, obsAt(start.Add(time.Duration(i)*30*time.Minute), float64(i)))
	}
	if _, err := s.Upsert(ctx, batch); err != nil {
		t.Fatal(err)
	}
	ser, err := s.Series(ctx, "temp", start, start.Add(48*time.Hour), 24)
	if err != nil {
		t.Fatal(err)
	}
	if len(ser.Points) == 0 || len(ser.Points) > 49 {
		t.Fatalf("got %d points, want a few dozen", len(ser.Points))
	}
	if !ser.HasData || ser.Min != 0 || ser.Max != 95 {
		t.Errorf("stats wrong: hasData=%v min=%v max=%v", ser.HasData, ser.Min, ser.Max)
	}
}

func TestSeriesRejectsUnknownElement(t *testing.T) {
	s := open(t)
	_, err := s.Series(context.Background(), "nope", time.Now(), time.Now().Add(time.Hour), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRecordsExcludeWindow(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	j := func(y int, temp float64) metar.Obs {
		return obsAt(time.Date(y, 1, 5, 12, 0, 0, 0, time.UTC), temp)
	}
	if _, err := s.Upsert(ctx, []metar.Obs{j(2019, 20), j(2020, 30), j(2021, 40)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshRecords(ctx); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC)
	ser, err := s.Series(ctx, "temp", from, from.Add(23*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if !ser.HasRecord || ser.RecordHigh != 30 || ser.RecordLow != 20 {
		t.Fatalf("record = %+v, want high 30 low 20", ser)
	}
	if ser.RecordHighDate != "2020-01-05" || ser.RecordLowDate != "2019-01-05" {
		t.Errorf("record dates = %s / %s", ser.RecordHighDate, ser.RecordLowDate)
	}
}

func TestMonthDone(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	if ok, _ := s.MonthDone(ctx, "2013-03"); ok {
		t.Fatal("month should not be done")
	}
	if err := s.MarkMonth(ctx, "2013-03"); err != nil {
		t.Fatal(err)
	}
	if ok, _ := s.MonthDone(ctx, "2013-03"); !ok {
		t.Fatal("month should be done")
	}
}

func TestSeriesEmptyDatabase(t *testing.T) {
	s := open(t)
	ser, err := s.Series(context.Background(), "temp", time.Now().Add(-24*time.Hour), time.Now(), 96)
	if err != nil {
		t.Fatal(err)
	}
	if ser.HasData || len(ser.Points) != 0 {
		t.Fatalf("expected empty series, got %+v", ser)
	}
}

func TestReparseRewritesParsedColumns(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	// Stored as an observation with no wind, as an older parser would have.
	if _, err := s.Upsert(ctx, []metar.Obs{{Time: time.Date(2012, 6, 1, 12, 0, 0, 0, time.UTC), Raw: "KM40 011200Z AUTO 10SM CLR 22/12 A3010"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reparse(ctx); err != nil {
		t.Fatal(err)
	}
	var wind sql.NullFloat64
	if err := s.db.QueryRow(`SELECT wind_kt FROM observations`).Scan(&wind); err != nil {
		t.Fatal(err)
	}
	if !wind.Valid || wind.Float64 != 0 {
		t.Errorf("wind = %v, want 0 (calm)", wind)
	}
}

func TestRecordsSnapshotRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	ctx := context.Background()
	s1, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	j := time.Date(2020, 1, 5, 12, 0, 0, 0, time.UTC)
	if _, err := s1.Upsert(ctx, []metar.Obs{obsAt(j, 30)}); err != nil {
		t.Fatal(err)
	}
	if err := s1.RefreshRecords(ctx); err != nil {
		t.Fatal(err)
	}
	s1.Close()

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if err := s2.LoadRecords(ctx); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC)
	ser, err := s2.Series(ctx, "temp", from, from.Add(23*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if !ser.HasRecord || ser.RecordHigh != 30 || ser.RecordHighDate != "2020-01-05" {
		t.Fatalf("snapshot not restored: %+v", ser)
	}
}

func TestSeriesExtremesUseRawObservations(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	start := time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)
	var batch []metar.Obs
	for i := 0; i < 48; i++ {
		batch = append(batch, obsAt(start.Add(time.Duration(i)*time.Hour), 20))
	}
	batch[30] = obsAt(start.Add(30*time.Hour), 35) // single-hour spike
	if _, err := s.Upsert(ctx, batch); err != nil {
		t.Fatal(err)
	}
	ser, err := s.Series(ctx, "temp", start, start.Add(47*time.Hour), 12)
	if err != nil {
		t.Fatal(err)
	}
	if ser.Max != 35 || !ser.MaxAt.Equal(batch[30].Time) {
		t.Errorf("max = %v at %v, want 35 at the spike hour", ser.Max, ser.MaxAt)
	}
	if ser.Min != 20 {
		t.Errorf("min = %v, want 20", ser.Min)
	}
}

func TestRecordTiesPickEarliestDate(t *testing.T) {
	s := open(t)
	ctx := context.Background()
	j := func(y int) metar.Obs {
		return obsAt(time.Date(y, 1, 5, 12, 0, 0, 0, time.UTC), 20)
	}
	if _, err := s.Upsert(ctx, []metar.Obs{j(2020), j(2013)}); err != nil {
		t.Fatal(err)
	}
	if err := s.RefreshRecords(ctx); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC)
	ser, err := s.Series(ctx, "temp", from, from.Add(23*time.Hour), 10)
	if err != nil {
		t.Fatal(err)
	}
	if ser.RecordHighDate != "2013-01-05" || ser.RecordLowDate != "2013-01-05" {
		t.Errorf("tied records = %s / %s, want the earliest date", ser.RecordHighDate, ser.RecordLowDate)
	}
}

// Months marked before the IEM end-date fix are missing their last day, so
// the first Open after the fix clears the marks once and only once.
func TestOpenClearsStaleBackfillMarksOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "m.db")
	ctx := context.Background()

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MarkMonth(ctx, "2020-01"); err != nil {
		t.Fatal(err)
	}
	// Simulate a database written before the fix: marks exist, no migration flag.
	if _, err := s.db.Exec(`DELETE FROM cache WHERE key = 'backfill_end_exclusive'`); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if done, _ := s.MonthDone(ctx, "2020-01"); done {
		t.Error("stale mark should have been cleared on first open after the fix")
	}
	if err := s.MarkMonth(ctx, "2020-01"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if done, _ := s.MonthDone(ctx, "2020-01"); !done {
		t.Error("marks written after the fix must survive a reopen")
	}
}
