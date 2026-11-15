package api

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"km40/internal/ingest"
	"km40/internal/metar"
	"km40/internal/store"
)

// Date-only parameters are station-local (America/Chicago) calendar days and
// the end date is inclusive: the whole of that day is in the window.
func TestWindowDatesAreStationLocalAndEndInclusive(t *testing.T) {
	cases := []struct {
		from, to         string
		wantFrom, wantTo string
	}{
		{"2024-01-05", "2024-01-05", "2024-01-05T06:00:00Z", "2024-01-06T05:59:59Z"},                     // CST
		{"2024-07-04", "2024-07-04", "2024-07-04T05:00:00Z", "2024-07-05T04:59:59Z"},                     // CDT
		{"2024-03-10", "2024-03-10", "2024-03-10T06:00:00Z", "2024-03-11T04:59:59Z"},                     // 23 h DST day
		{"2024-11-03", "2024-11-03", "2024-11-03T05:00:00Z", "2024-11-04T05:59:59Z"},                     // 25 h DST day
		{"2024-01-05T10:00:00Z", "2024-01-06T10:00:00Z", "2024-01-05T10:00:00Z", "2024-01-06T10:00:00Z"}, // instants pass through
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/api/series?from="+c.from+"&to="+c.to, nil)
		from, to, _, err := window(r, 100)
		if err != nil {
			t.Fatalf("%s..%s: %v", c.from, c.to, err)
		}
		if got := from.UTC().Format(time.RFC3339); got != c.wantFrom {
			t.Errorf("from %s = %s, want %s", c.from, got, c.wantFrom)
		}
		if got := to.UTC().Format(time.RFC3339); got != c.wantTo {
			t.Errorf("to %s = %s, want %s", c.to, got, c.wantTo)
		}
	}
}

func testHandler(t *testing.T) (http.Handler, *store.Store) {
	t.Helper()
	return testHandlerWith(t, nil)
}

func testHandlerWith(t *testing.T, health HealthSource) (http.Handler, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	static := fstest.MapFS{
		"index.html":  &fstest.MapFile{Data: []byte("<html>index</html>")},
		"200.html":    &fstest.MapFile{Data: []byte("<html>spa</html>")},
		"_app/app.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	}
	return New(st, static, health), st
}

type fakeHealth struct{ s ingest.Snapshot }

func (f fakeHealth) Snapshot() ingest.Snapshot { return f.s }

// /api/status reports backfill and poller health when ingest is running.
func TestStatusIncludesIngestHealth(t *testing.T) {
	h, _ := testHandlerWith(t, fakeHealth{ingest.Snapshot{
		Backfill: ingest.BackfillStatus{State: "running", MonthsDone: 12, MonthsTotal: 178},
		Poller:   ingest.PollerStatus{LastSuccessAt: "2026-11-15T12:00:00Z", LastReports: 4, IntervalSeconds: 300, LastError: "boom"},
	}})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var got struct {
		Ingest *struct {
			Backfill struct {
				State       string `json:"state"`
				MonthsDone  int    `json:"monthsDone"`
				MonthsTotal int    `json:"monthsTotal"`
			} `json:"backfill"`
			Poller struct {
				LastSuccessAt   string `json:"lastSuccessAt"`
				LastReports     int    `json:"lastReports"`
				IntervalSeconds int    `json:"intervalSeconds"`
				LastError       string `json:"lastError"`
			} `json:"poller"`
		} `json:"ingest"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Ingest == nil {
		t.Fatalf("no ingest health in %s", rec.Body.String())
	}
	if b := got.Ingest.Backfill; b.State != "running" || b.MonthsDone != 12 || b.MonthsTotal != 178 {
		t.Errorf("backfill = %+v", b)
	}
	if p := got.Ingest.Poller; p.LastSuccessAt != "2026-11-15T12:00:00Z" || p.LastReports != 4 || p.IntervalSeconds != 300 || p.LastError != "boom" {
		t.Errorf("poller = %+v", p)
	}
}

// With -no-ingest there is nothing to report; the field is null, not an error.
func TestStatusWithoutIngestHealth(t *testing.T) {
	h, _ := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var got map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if v, ok := got["ingest"]; !ok || string(v) != "null" {
		t.Errorf("ingest = %s (present=%v), want null", v, ok)
	}
}

func TestSummaryWorksOnEmptyDatabase(t *testing.T) {
	h, _ := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/summary", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var got Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Elements) != 11 {
		t.Fatalf("elements = %d, want 11", len(got.Elements))
	}
	if got.Station.ID != "KM40" {
		t.Errorf("station = %q", got.Station.ID)
	}
}

func TestSummaryIncludesCurrentAndRecords(t *testing.T) {
	h, st := testHandler(t)
	temp := 21.0
	obs := metar.Obs{Time: timeFromNow(-10), Raw: "KM40 141000Z 21012G20KT 10SM BKN045 21/14 A3002", TempC: &temp}
	dew := 14.0
	obs.DewpointC = &dew
	if _, err := st.Upsert(t.Context(), []metar.Obs{obs}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/summary", nil))
	var got Summary
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Current.TempC == nil || *got.Current.TempC != 21 || got.Current.Humidity == nil {
		t.Errorf("current = %+v", got.Current)
	}
	if got.Raw == "" || got.ObservedAt == "" {
		t.Errorf("missing raw/observedAt: %+v", got)
	}
}

func TestSeriesRejectsUnknownElement(t *testing.T) {
	h, _ := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/series?element=nope", nil))
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "temp") {
		t.Errorf("error should list valid elements: %s", rec.Body.String())
	}
}

// A database failure is the server's fault, not a bad request: only an
// unknown element or bad parameter is a 400.
func TestSeriesDatabaseFailureIsServerError(t *testing.T) {
	h, st := testHandler(t)
	st.Close()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/series?element=temp", nil))
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

func TestStatusDatabaseFailureIsServerError(t *testing.T) {
	h, st := testHandler(t)
	st.Close()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// A value that cannot be encoded must become an error response, not a silent
// 200 with an empty body.
func TestWriteJSONReportsEncodeFailure(t *testing.T) {
	rec := httptest.NewRecorder()
	writeJSON(rec, make(chan int))
	if rec.Code != 500 {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// A directory without an index is not browsable: it falls back to the app
// shell like any other unknown path.
func TestStaticDirectoryIsNotListed(t *testing.T) {
	h, _ := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/_app/", nil))
	body, _ := io.ReadAll(rec.Body)
	if strings.Contains(string(body), "app.js") {
		t.Errorf("directory listing exposed: %q", body)
	}
	if !strings.Contains(string(body), "spa") {
		t.Errorf("want the app shell, got %q", body)
	}
}

func TestSeriesRejectsBadDates(t *testing.T) {
	h, _ := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/series?element=temp&from=yesterday", nil))
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestLatestWorksWhenEmptyAndPopulated(t *testing.T) {
	h, st := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/latest", nil))
	if rec.Code != 200 {
		t.Fatalf("empty db status = %d", rec.Code)
	}
	temp := 18.0
	if _, err := st.Upsert(t.Context(), []metar.Obs{{Time: timeFromNow(-3), Raw: "KM40 141000Z 10SM CLR 18/10 A3005", TempC: &temp}}); err != nil {
		t.Fatal(err)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/latest", nil))
	var got struct {
		ObservedAt string  `json:"observedAt"`
		Current    Current `json:"current"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Current.TempC == nil || *got.Current.TempC != 18 || got.ObservedAt == "" {
		t.Errorf("latest = %+v", got)
	}
}

func TestLatestReportsSkyCoverPercent(t *testing.T) {
	h, st := testHandler(t)
	// Latest decodes the stored raw text, so it needs a real report.
	raw := "KM40 141000Z AUTO 00000KT 10SM FEW017 BKN048 18/10 A3005"
	if _, err := st.Upsert(t.Context(), []metar.Obs{{Time: timeFromNow(-3), Raw: raw}}); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/latest", nil))
	var got struct {
		Current Current `json:"current"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Current.SkyCoverPct == nil || *got.Current.SkyCoverPct != 75 {
		t.Errorf("skyCoverPct = %v, want 75", got.Current.SkyCoverPct)
	}
}

func TestStatusReportsCounts(t *testing.T) {
	h, _ := testHandler(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/status", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var got Status
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Observations != 0 || len(got.Elements) != 11 {
		t.Errorf("status = %+v", got)
	}
}

func TestStaticAndSPAFallback(t *testing.T) {
	h, _ := testHandler(t)
	for _, tc := range []struct{ path, want string }{
		{"/", "index"},
		{"/_app/app.js", "console"},
		{"/element/temp", "spa"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", tc.path, nil))
		body, _ := io.ReadAll(rec.Body)
		if rec.Code != 200 || !strings.Contains(string(body), tc.want) {
			t.Errorf("%s: status %d body %q", tc.path, rec.Code, body)
		}
	}
}

func timeFromNow(minutes int) (t time.Time) {
	return time.Now().UTC().Add(time.Duration(minutes) * time.Minute)
}
