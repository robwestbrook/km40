// Package api serves the JSON endpoints and the embedded SvelteKit build.
package api

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"km40/internal/metar"
	"km40/internal/store"
)

type Station struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	Elevation int     `json:"elevationFt"`
	Timezone  string  `json:"timezone"`
}

var station = Station{
	ID:        "KM40",
	Name:      "Aberdeen / Monroe County Airport",
	Lat:       33.8738,
	Lon:       -88.4897,
	Elevation: 68,
	Timezone:  "America/Chicago",
}

// Current is the latest observation in stored units; the UI converts for display.
type Current struct {
	TempC         *float64 `json:"tempC"`
	DewpointC     *float64 `json:"dewpointC"`
	Humidity      *float64 `json:"humidity"`
	WindDir       *int     `json:"windDir"`
	WindKt        *float64 `json:"windKt"`
	GustKt        *float64 `json:"gustKt"`
	VisibilitySM  *float64 `json:"visibilitySM"`
	Sky           string   `json:"sky"`
	CeilingFt     *int     `json:"ceilingFt"`
	Weather       string   `json:"weather"`
	AltimeterInHg *float64 `json:"altimeterInHg"`
	PrecipIn      *float64 `json:"precipIn"`
}

type Summary struct {
	Station    Station        `json:"station"`
	ObservedAt string         `json:"observedAt"`
	AgeMinutes int            `json:"ageMinutes"`
	Raw        string         `json:"raw"`
	Current    Current        `json:"current"`
	From       string         `json:"from"`
	To         string         `json:"to"`
	Elements   []store.Series `json:"elements"`
}

type Status struct {
	Observations int      `json:"observations"`
	First        string   `json:"first"`
	Last         string   `json:"last"`
	Years        []int    `json:"years"`
	Elements     []string `json:"elements"`
}

func New(st *store.Store, static fs.FS) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/summary", func(w http.ResponseWriter, r *http.Request) {
		from, to, points, err := window(r, 96)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		out := Summary{
			Station: station,
			From:    from.Format(time.RFC3339),
			To:      to.Format(time.RFC3339),
		}
		latest, err := st.Latest(r.Context())
		if err != nil {
			httpErr(w, err)
			return
		}
		if latest != nil {
			out.ObservedAt = latest.Time.Format(time.RFC3339)
			out.AgeMinutes = int(time.Since(latest.Time).Minutes())
			out.Raw = latest.Raw
			out.Current = currentOf(latest)
		}
		// One query per element; run them together so a wide window does not
		// multiply the wait by nine.
		els := store.Elements()
		sers := make([]store.Series, len(els))
		var wg sync.WaitGroup
		var mu sync.Mutex
		var firstErr error
		for i, e := range els {
			wg.Add(1)
			go func(i int, e store.Element) {
				defer wg.Done()
				ser, err := st.Series(r.Context(), e.Key, from, to, points)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					if firstErr == nil {
						firstErr = err
					}
					return
				}
				sers[i] = ser
			}(i, e)
		}
		wg.Wait()
		if firstErr != nil {
			httpErr(w, firstErr)
			return
		}
		out.Elements = sers
		writeJSON(w, out)
	})

	mux.HandleFunc("GET /api/series", func(w http.ResponseWriter, r *http.Request) {
		from, to, points, err := window(r, 240)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		key := r.URL.Query().Get("element")
		ser, err := st.Series(r.Context(), key, from, to, points)
		if err != nil {
			http.Error(w, fmt.Sprintf("%v (valid elements: %s)", err, store.ElementKeys()), http.StatusBadRequest)
			return
		}
		writeJSON(w, ser)
	})

	mux.HandleFunc("GET /api/latest", func(w http.ResponseWriter, r *http.Request) {
		out := struct {
			ObservedAt string  `json:"observedAt"`
			AgeMinutes int     `json:"ageMinutes"`
			Raw        string  `json:"raw"`
			Current    Current `json:"current"`
		}{}
		latest, err := st.Latest(r.Context())
		if err != nil {
			httpErr(w, err)
			return
		}
		if latest != nil {
			out.ObservedAt = latest.Time.Format(time.RFC3339)
			out.AgeMinutes = int(time.Since(latest.Time).Minutes())
			out.Raw = latest.Raw
			out.Current = currentOf(latest)
		}
		writeJSON(w, out)
	})

	mux.HandleFunc("GET /api/status", func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		out := Status{}
		out.Observations, _ = st.Count(ctx)
		first, last, err := st.Span(ctx)
		if err != nil {
			httpErr(w, err)
			return
		}
		if !first.IsZero() {
			out.First = first.Format(time.RFC3339)
			out.Last = last.Format(time.RFC3339)
			for y := first.Year(); y <= last.Year(); y++ {
				out.Years = append(out.Years, y)
			}
		}
		for _, e := range store.Elements() {
			out.Elements = append(out.Elements, e.Key)
		}
		writeJSON(w, out)
	})

	mux.Handle("/", staticHandler(static))
	return mux
}

func currentOf(o *metar.Obs) Current {
	c := Current{
		TempC:         o.TempC,
		DewpointC:     o.DewpointC,
		WindDir:       o.WindDir,
		WindKt:        o.WindKt,
		GustKt:        o.GustKt,
		VisibilitySM:  o.VisSM,
		Sky:           o.Sky,
		CeilingFt:     o.CeilingFt,
		Weather:       o.Wx,
		AltimeterInHg: o.AltimeterInHg,
		PrecipIn:      o.PrecipIn,
	}
	if o.TempC != nil && o.DewpointC != nil {
		rh := metar.RelativeHumidity(*o.TempC, *o.DewpointC)
		c.Humidity = &rh
	}
	return c
}

// window reads from/to/points with defaults. Dates are "2006-01-02" (UTC) or RFC3339.
func window(r *http.Request, defaultPoints int) (time.Time, time.Time, int, error) {
	q := r.URL.Query()
	to := time.Now().UTC()
	from := to.Add(-24 * time.Hour)
	parse := func(s string) (time.Time, error) {
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t, nil
		}
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return time.Time{}, fmt.Errorf("bad date %q: use YYYY-MM-DD", s)
		}
		return t, nil
	}
	var err error
	if v := q.Get("to"); v != "" {
		if to, err = parse(v); err != nil {
			return from, to, 0, err
		}
	}
	if v := q.Get("from"); v != "" {
		if from, err = parse(v); err != nil {
			return from, to, 0, err
		}
	}
	if from.After(to) {
		return from, to, 0, fmt.Errorf("from must be before to")
	}
	points := defaultPoints
	if v := q.Get("points"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 10 || n > 2000 {
			return from, to, 0, fmt.Errorf("points must be 10..2000")
		}
		points = n
	}
	return from, to, points, nil
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(v)
}

func httpErr(w http.ResponseWriter, err error) {
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

// staticHandler serves the SvelteKit build, falling back to the SPA shell so
// client-side routes such as /element/temp work on a hard refresh.
func staticHandler(fsys fs.FS) http.Handler {
	files := http.FileServer(http.FS(fsys))
	shell := "200.html"
	if _, err := fs.Stat(fsys, shell); err != nil {
		shell = "index.html"
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
		if name == "" || name == "." {
			name = "index.html"
		}
		if _, err := fs.Stat(fsys, name); err != nil {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				http.NotFound(w, r)
				return
			}
			r = r.Clone(r.Context())
			r.URL.Path = "/" + shell
		}
		files.ServeHTTP(w, r)
	})
}
