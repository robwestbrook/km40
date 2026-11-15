package ingest

import (
	"sync"
	"time"
)

// BackfillStatus is the progress of the one-time archive load.
type BackfillStatus struct {
	// State is "pending" (not started), "running", "done" or "failed".
	State       string `json:"state"`
	MonthsDone  int    `json:"monthsDone"`
	MonthsTotal int    `json:"monthsTotal"`
	LastError   string `json:"lastError"`
	LastErrorAt string `json:"lastErrorAt"`
}

// PollerStatus is the state of the live feed. LastReports is from the last
// successful poll and is kept when a later poll fails.
type PollerStatus struct {
	LastAttemptAt   string `json:"lastAttemptAt"`
	LastSuccessAt   string `json:"lastSuccessAt"`
	LastError       string `json:"lastError"`
	LastErrorAt     string `json:"lastErrorAt"`
	LastReports     int    `json:"lastReports"`
	IntervalSeconds int    `json:"intervalSeconds"`
}

// Snapshot is a copy of the ingest state, safe to serve as JSON. Times are
// RFC3339 UTC, empty until the event has happened.
type Snapshot struct {
	Backfill BackfillStatus `json:"backfill"`
	Poller   PollerStatus   `json:"poller"`
}

// Health records what the backfill and the poller are doing, so a stalled
// feed or a failed backfill can be seen from /api/status instead of the log.
// A nil *Health is valid and records nothing (ingest turned off).
type Health struct {
	mu sync.Mutex
	s  Snapshot
}

func NewHealth() *Health {
	return &Health{s: Snapshot{
		Backfill: BackfillStatus{State: "pending"},
		Poller:   PollerStatus{IntervalSeconds: int(pollEvery / time.Second)},
	}}
}

// Snapshot returns a copy of the current state.
func (h *Health) Snapshot() Snapshot {
	if h == nil {
		return Snapshot{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.s
}

func (h *Health) update(fn func(*Snapshot)) {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	fn(&h.s)
}

func stamp() string { return time.Now().UTC().Format(time.RFC3339) }
