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
