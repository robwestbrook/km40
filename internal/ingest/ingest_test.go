package ingest

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	if err := Backfill(ctx, s, srv.Client(), from, to, nil); err != nil {
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

func TestParseIEMPreservesCommaInRemarks(t *testing.T) {
	obs := parseIEM([]byte("station,valid,metar\nM40,2013-03-01 00:15,KM40 010015Z AUTO 34007KT 10SM OVC046 06/05 A3010 RMK AO2 P0000,SLP123\n"))
	if len(obs) != 1 {
		t.Fatalf("got %d obs", len(obs))
	}
	if !strings.Contains(obs[0].Raw, "SLP123") {
		t.Errorf("raw lost text after comma: %q", obs[0].Raw)
	}
}
