// Package ingest keeps the observation table current: one initial backfill
// from the IEM archive, then the live Aviation Weather Center feed.
package ingest

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"km40/internal/metar"
	"km40/internal/store"
)

// Endpoints are variables so tests can point them at httptest servers.
var (
	AWCEndpoint = "https://aviationweather.gov/api/data/metar?ids=KM40&format=raw&hours=6"
	IEMEndpoint = "https://mesonet.agron.iastate.edu/cgi-bin/request/asos.py"
	// ArchiveStart is the first day IEM has data for M40.
	ArchiveStart = time.Date(2012, 1, 11, 0, 0, 0, 0, time.UTC)
)

const userAgent = "km40-dashboard/1.0 (+personal weather dashboard)"

// pollEvery is how often the live feed is fetched.
const pollEvery = 5 * time.Minute

// retryWait is the first pause between retries of a failed archive request
// (it quadruples each time); a variable so tests need not wait.
var retryWait = 5 * time.Second

func newRequest(ctx context.Context, rawURL string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	return req, nil
}

func get(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	req, err := newRequest(ctx, rawURL)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ingest: %s: %s", rawURL, resp.Status)
	}
	if strings.Contains(string(body), "Too many requests") {
		return nil, fmt.Errorf("ingest: rate limited by %s", rawURL)
	}
	return body, nil
}

// Live fetches the last few hours of METARs and upserts them.
func Live(ctx context.Context, st *store.Store, client *http.Client) (int, error) {
	body, err := get(ctx, client, AWCEndpoint)
	if err != nil {
		return 0, err
	}
	now := time.Now().UTC()
	var obs []metar.Obs
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "METAR ") && !strings.HasPrefix(line, "SPECI ") {
			continue
		}
		o, err := metar.Parse(line, now)
		if err != nil {
			continue // one bad report must not stop the batch
		}
		obs = append(obs, o)
	}
	if len(obs) == 0 {
		return 0, nil
	}
	return st.Upsert(ctx, obs)
}

// Backfill fetches raw METARs month by month. Completed months are marked, so
// an interrupted run resumes where it stopped.
//
// health, which may be nil, is told how far the run has got and why it failed.
func Backfill(ctx context.Context, st *store.Store, client *http.Client, from, to time.Time, log func(string), health *Health) error {
	from = time.Date(from.Year(), from.Month(), 1, 0, 0, 0, 0, time.UTC)
	total := 0
	for m := from; m.Before(to); m = m.AddDate(0, 1, 0) {
		total++
	}
	health.update(func(s *Snapshot) { s.Backfill = BackfillStatus{State: "running", MonthsTotal: total} })
	// fail records an error unless the run was merely cancelled (shutdown).
	fail := func(err error) error {
		if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
			health.update(func(s *Snapshot) {
				s.Backfill.State, s.Backfill.LastError, s.Backfill.LastErrorAt = "failed", err.Error(), stamp()
			})
		}
		return err
	}
	complete := 0
	monthComplete := func() {
		complete++
		health.update(func(s *Snapshot) { s.Backfill.MonthsDone = complete })
	}
	for m := from; m.Before(to); m = m.AddDate(0, 1, 0) {
		ym := m.Format("2006-01")
		if done, err := st.MonthDone(ctx, ym); err != nil {
			return fail(err)
		} else if done {
			monthComplete()
			continue
		}
		// IEM's end date is exclusive: end on the first of the next month, or
		// the month's last day is dropped.
		next := m.AddDate(0, 1, 0)
		q := url.Values{
			"station": {"M40"},
			"data":    {"metar"},
			"year1":   {m.Format("2006")}, "month1": {m.Format("1")}, "day1": {"1"},
			"year2": {next.Format("2006")}, "month2": {next.Format("1")}, "day2": {"1"},
			"tz": {"UTC"}, "format": {"onlycomma"}, "latlon": {"no"},
			"missing": {"M"}, "trace": {"T"}, "direct": {"no"},
		}
		body, err := getWithRetry(ctx, client, IEMEndpoint+"?"+q.Encode())
		if err != nil {
			return fail(fmt.Errorf("backfill %s: %w", ym, err))
		}
		obs := parseIEM(body)
		if len(obs) > 0 {
			if _, err := st.Upsert(ctx, obs); err != nil {
				return fail(fmt.Errorf("backfill %s: %w", ym, err))
			}
		}
		// Only a finished month is marked done; an in-progress month is
		// refetched on the next run so downtime never leaves a permanent gap.
		if !next.After(to) {
			if err := st.MarkMonth(ctx, ym); err != nil {
				return fail(err)
			}
		}
		monthComplete()
		if log != nil {
			log(fmt.Sprintf("backfilled %s (%d reports)", ym, len(obs)))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
	health.update(func(s *Snapshot) { s.Backfill.State = "done" })
	return nil
}

// getWithRetry retries transient failures (IEM throttles bursts) before
// giving up; the month stays unmarked, so a restart resumes it.
func getWithRetry(ctx context.Context, client *http.Client, rawURL string) ([]byte, error) {
	var err error
	for attempt, wait := 0, retryWait; attempt < 3; attempt++ {
		var body []byte
		if body, err = get(ctx, client, rawURL); err == nil {
			return body, nil
		}
		if attempt == 2 {
			break
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
		wait *= 4
	}
	return nil, err
}

// parseIEM reads IEM's CSV: station,valid,metar. The METAR is the rest of the
// line, so any comma inside remarks is preserved.
func parseIEM(body []byte) []metar.Obs {
	var obs []metar.Obs
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		parts := strings.SplitN(line, ",", 3)
		if len(parts) != 3 || parts[1] == "valid" {
			continue
		}
		valid, err := time.Parse("2006-01-02 15:04", parts[1])
		if err != nil {
			continue
		}
		o, err := metar.Parse(parts[2], valid)
		if err != nil {
			if o.Raw == "" {
				continue // nothing to keep
			}
			// IEM gave us the time, so keep the report (raw only) rather than
			// lose it; a later parser fix can decode it with -reparse.
			o.Time = valid
		}
		obs = append(obs, o)
	}
	return obs
}

// poll fetches the live feed once, records the outcome in health and returns
// the number of reports stored. A failed poll keeps the last success on record.
func poll(ctx context.Context, st *store.Store, client *http.Client, health *Health, log func(string)) int {
	health.update(func(s *Snapshot) { s.Poller.LastAttemptAt = stamp() })
	n, err := Live(ctx, st, client)
	if err != nil {
		if log != nil {
			log("live: " + err.Error())
		}
		if ctx.Err() == nil {
			health.update(func(s *Snapshot) { s.Poller.LastError, s.Poller.LastErrorAt = err.Error(), stamp() })
		}
		return 0
	}
	health.update(func(s *Snapshot) {
		s.Poller.LastSuccessAt, s.Poller.LastReports, s.Poller.LastError = stamp(), n, ""
	})
	return n
}

// Run backfills history, then polls the live feed every five minutes. Record
// tables are refreshed after the backfill and hourly.
func Run(ctx context.Context, st *store.Store, client *http.Client, health *Health, log func(string)) error {
	if log != nil {
		log("backfill starting")
	}
	before, _ := st.Count(ctx)
	if err := Backfill(ctx, st, client, ArchiveStart, time.Now().UTC(), log, health); err != nil {
		if log != nil {
			log("backfill: " + err.Error())
		}
	}
	// Records are only rebuilt when the backfill actually added history.
	if after, _ := st.Count(ctx); after > before {
		if err := st.RefreshRecords(ctx); err != nil && log != nil {
			log("records: " + err.Error())
		}
	}
	if log != nil {
		log("backfill complete; polling live feed")
	}

	lastRefresh := time.Now()
	ticker := time.NewTicker(pollEvery)
	defer ticker.Stop()
	for {
		n := poll(ctx, st, client, health, log)
		if n > 0 && time.Since(lastRefresh) > time.Hour {
			// Count the attempt even when it fails (for example a snapshot
			// that cannot be saved): retrying a 30 s scan every poll would
			// hammer the drive. The next try is an hour away.
			lastRefresh = time.Now()
			if err := st.RefreshRecords(ctx); err != nil && log != nil {
				log("records: " + err.Error())
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
