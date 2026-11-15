package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"km40/internal/store"
)

func openStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestLiveUpsertsAndSkipsGarbage(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("METAR KM40 150115Z AUTO 00000KT 10SM CLR 04/00 A3028 RMK AO2\n" +
			"this line is not a metar\n" +
			"METAR KM40 150035Z AUTO 00000KT 10SM CLR 04/M02 A3027 RMK AO2\n"))
	}))
	defer srv.Close()
	old := AWCEndpoint
	AWCEndpoint = srv.URL
	defer func() { AWCEndpoint = old }()

	s := openStore(t)
	n, err := Live(context.Background(), s, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("upserted %d, want 2", n)
	}
	if c, _ := s.Count(context.Background()); c != 2 {
		t.Errorf("count = %d, want 2", c)
	}
}

func TestLiveRejectsErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()
	old := AWCEndpoint
	AWCEndpoint = srv.URL
	defer func() { AWCEndpoint = old }()

	if _, err := Live(context.Background(), openStore(t), srv.Client()); err == nil {
		t.Fatal("expected an error")
	}
}

func TestBackfillSkipsDoneMonths(t *testing.T) {
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Query().Get("year1")+"-"+r.URL.Query().Get("month1"))
		w.Write([]byte("station,valid,metar\n" +
			"M40,2013-03-01 00:15,KM40 010015Z AUTO 34007KT 10SM OVC046 06/05 A3010 RMK AO2\n"))
	}))
	defer srv.Close()
	old := IEMEndpoint
	IEMEndpoint = srv.URL
	defer func() { IEMEndpoint = old }()

	s := openStore(t)
	ctx := context.Background()
	if err := s.MarkMonth(ctx, "2013-03"); err != nil {
		t.Fatal(err)
	}
	from := time.Date(2013, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2013, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := Backfill(ctx, s, srv.Client(), from, to, nil, nil); err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0] != "2013-4" {
		t.Errorf("requested months = %v, want only 2013-4", hits)
	}
	if c, _ := s.Count(ctx); c != 1 {
		t.Errorf("count = %d, want 1", c)
	}
	if done, _ := s.MonthDone(ctx, "2013-04"); !done {
		t.Error("April should be marked done")
	}
}

// /api/status reports backfill progress: months done out of months in range,
// counting months that were already complete.
func TestBackfillReportsProgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("station,valid,metar\n"))
	}))
	defer srv.Close()
	old := IEMEndpoint
	IEMEndpoint = srv.URL
	defer func() { IEMEndpoint = old }()

	s := openStore(t)
	ctx := context.Background()
	if err := s.MarkMonth(ctx, "2013-03"); err != nil {
		t.Fatal(err)
	}
	h := NewHealth()
	if got := h.Snapshot().Backfill.State; got != "pending" {
		t.Fatalf("initial state = %q, want pending", got)
	}
	from := time.Date(2013, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2013, 5, 1, 0, 0, 0, 0, time.UTC)
	if err := Backfill(ctx, s, srv.Client(), from, to, nil, h); err != nil {
		t.Fatal(err)
	}
	b := h.Snapshot().Backfill
	if b.State != "done" || b.MonthsDone != 2 || b.MonthsTotal != 2 || b.LastError != "" {
		t.Errorf("backfill = %+v, want done 2/2 with no error", b)
	}
}

func TestBackfillFailureIsRecorded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadGateway)
	}))
	defer srv.Close()
	oldEP, oldWait := IEMEndpoint, retryWait
	IEMEndpoint, retryWait = srv.URL, time.Millisecond
	defer func() { IEMEndpoint, retryWait = oldEP, oldWait }()

	h := NewHealth()
	from := time.Date(2013, 3, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2013, 4, 1, 0, 0, 0, 0, time.UTC)
	if err := Backfill(context.Background(), openStore(t), srv.Client(), from, to, nil, h); err == nil {
		t.Fatal("expected an error")
	}
	b := h.Snapshot().Backfill
	if b.State != "failed" || !strings.Contains(b.LastError, "2013-03") || b.LastErrorAt == "" || b.MonthsDone != 0 || b.MonthsTotal != 1 {
		t.Errorf("backfill = %+v, want failed on 2013-03 at 0/1", b)
	}
}

// The poller records when it last tried, when it last succeeded and the last
// error, so a stalled feed is visible without reading logs.
func TestPollRecordsSuccessAndFailure(t *testing.T) {
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if fail {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		w.Write([]byte("METAR KM40 150115Z AUTO 00000KT 10SM CLR 04/00 A3028 RMK AO2\n" +
			"METAR KM40 150035Z AUTO 00000KT 10SM CLR 04/M02 A3027 RMK AO2\n"))
	}))
	defer srv.Close()
	old := AWCEndpoint
	AWCEndpoint = srv.URL
	defer func() { AWCEndpoint = old }()

	s := openStore(t)
	h := NewHealth()
	if n := poll(context.Background(), s, srv.Client(), h, nil); n != 2 {
		t.Fatalf("poll fetched %d, want 2", n)
	}
	p := h.Snapshot().Poller
	if p.LastAttemptAt == "" || p.LastSuccessAt == "" || p.LastReports != 2 || p.LastError != "" || p.IntervalSeconds != 300 {
		t.Fatalf("after success: %+v", p)
	}
	okAt := p.LastSuccessAt

	fail = true
	poll(context.Background(), s, srv.Client(), h, nil)
	p = h.Snapshot().Poller
	if p.LastError == "" || p.LastErrorAt == "" {
		t.Errorf("failure not recorded: %+v", p)
	}
	if p.LastSuccessAt != okAt {
		t.Errorf("a failed poll must not move LastSuccessAt: %q -> %q", okAt, p.LastSuccessAt)
	}
}

// With ingest turned off there is no Health; every method must tolerate nil.
func TestNilHealthIsSafe(t *testing.T) {
	var h *Health
	h.update(func(s *Snapshot) { s.Backfill.State = "x" })
	if got := h.Snapshot(); got.Backfill.State != "" {
		t.Errorf("nil Health returned %+v", got)
	}
}

func TestParseIEMPreservesCommaInRemarks(t *testing.T) {
	obs := parseIEM([]byte("station,valid,metar\nM40,2013-03-01 00:15,KM40 010015Z AUTO 34007KT 10SM OVC046 06/05 A3010 RMK AO2 P0000,SLP123\n"))
	if len(obs) != 1 {
		t.Fatalf("got %d obs", len(obs))
	}
	if !strings.Contains(obs[0].Raw, "SLP123") {
		t.Errorf("raw lost text after comma: %q", obs[0].Raw)
	}
}

// IEM supplies the observation time in its own column, so a report whose
// METAR text has no decodable time group must still be stored (raw only).
func TestParseIEMKeepsReportWithoutTimeGroup(t *testing.T) {
	obs := parseIEM([]byte("station,valid,metar\n" +
		"M40,2013-03-01 00:15,KM40 AUTO 34007KT 10SM OVC046 06/05 A3010 RMK AO2\n" +
		"M40,2013-03-01 00:35,\n"))
	if len(obs) != 1 {
		t.Fatalf("got %d obs, want 1 (the empty report has nothing to keep)", len(obs))
	}
	want := time.Date(2013, 3, 1, 0, 15, 0, 0, time.UTC)
	if !obs[0].Time.Equal(want) {
		t.Errorf("time = %v, want IEM's %v", obs[0].Time, want)
	}
	if !strings.Contains(obs[0].Raw, "OVC046") {
		t.Errorf("raw text lost: %q", obs[0].Raw)
	}
}

// IEM treats the end date as exclusive, so a month request must end on the
// first day of the next month or the month's last day is silently dropped.
func TestBackfillRequestEndsOnFirstOfNextMonth(t *testing.T) {
	var got url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.Query()
		w.Write([]byte("station,valid,metar\n"))
	}))
	defer srv.Close()
	old := IEMEndpoint
	IEMEndpoint = srv.URL
	defer func() { IEMEndpoint = old }()

	from := time.Date(2013, 12, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2014, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := Backfill(context.Background(), openStore(t), srv.Client(), from, to, nil, nil); err != nil {
		t.Fatal(err)
	}
	for k, want := range map[string]string{
		"year1": "2013", "month1": "12", "day1": "1",
		"year2": "2014", "month2": "1", "day2": "1",
	} {
		if got.Get(k) != want {
			t.Errorf("%s = %q, want %q", k, got.Get(k), want)
		}
	}
}

// A month that is still in progress must be refetched next time, not marked
// done: the live feed only covers six hours, so a marked partial month would
// keep any gap forever.
func TestBackfillDoesNotMarkInProgressMonth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("station,valid,metar\n"))
	}))
	defer srv.Close()
	old := IEMEndpoint
	IEMEndpoint = srv.URL
	defer func() { IEMEndpoint = old }()

	s := openStore(t)
	ctx := context.Background()
	from := time.Date(2013, 3, 1, 0, 0, 0, 0, time.UTC)
	now := time.Date(2013, 3, 20, 12, 0, 0, 0, time.UTC)
	if err := Backfill(ctx, s, srv.Client(), from, now, nil, nil); err != nil {
		t.Fatal(err)
	}
	if done, _ := s.MonthDone(ctx, "2013-03"); done {
		t.Error("in-progress month must not be marked done")
	}
}
