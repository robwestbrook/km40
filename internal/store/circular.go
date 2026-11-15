package store

import (
	"context"
	"fmt"
	"math"
	"time"
)

// bearing turns mean sine and cosine components into a compass bearing in
// [0, 360). ok is false when the directions largely cancel (the mean vector is
// short), because the resulting bearing would be meaningless.
func bearing(sinAvg, cosAvg float64) (deg float64, ok bool) {
	if math.Hypot(sinAvg, cosAvg) < 0.1 {
		return 0, false
	}
	deg = math.Atan2(sinAvg, cosAvg) * 180 / math.Pi
	if deg < 0 {
		deg += 360
	}
	deg = math.Round(deg*10) / 10
	if deg >= 360 {
		deg -= 360
	}
	return deg, true
}

// circularSeries is Series for a bearing element. Averaging degrees directly
// would put 350 and 10 at 180, so each bucket (and the window) is the mean of
// the unit vectors instead. Reports without a direction (calm, variable) are
// not counted.
func (s *Store) circularSeries(ctx context.Context, e Element, from, to time.Time, bucket int64) (Series, error) {
	out := Series{Element: e.Key, Unit: e.Unit, Precision: e.Precision, Aggregate: e.agg, Points: []Point{}}

	rows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT (obs_time/?)*? b, AVG(sin(radians(%[1]s))), AVG(cos(radians(%[1]s)))
		 FROM observations
		 WHERE obs_time BETWEEN ? AND ? AND %[1]s IS NOT NULL
		 GROUP BY b ORDER BY b`, e.column), bucket, bucket, from.Unix(), to.Unix())
	if err != nil {
		return Series{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var sinAvg, cosAvg float64
		if err := rows.Scan(&b, &sinAvg, &cosAvg); err != nil {
			return Series{}, err
		}
		deg, ok := bearing(sinAvg, cosAvg)
		if !ok {
			continue
		}
		t := time.Unix(b, 0).UTC()
		if t.Before(from) {
			t = from // bucket floor can precede the requested start
		}
		out.Points = append(out.Points, Point{T: t, V: deg})
	}
	if err := rows.Err(); err != nil {
		return Series{}, err
	}
	out.HasData = len(out.Points) > 0

	// The prevailing direction is the compass point the wind comes from most
	// often. A vector mean would be undefined whenever winds come from all
	// around, which is normal over a long window. Sectors are 22.5 degrees wide,
	// centred on the 16 compass points; a tie goes to the lower point.
	srows, err := s.db.QueryContext(ctx, fmt.Sprintf(
		`SELECT CAST((%[1]s + 11.25) / 22.5 AS INTEGER) %% 16 sector, COUNT(*)
		 FROM observations WHERE obs_time BETWEEN ? AND ? AND %[1]s IS NOT NULL
		 GROUP BY sector ORDER BY sector`, e.column), from.Unix(), to.Unix())
	if err != nil {
		return Series{}, err
	}
	defer srows.Close()
	total, bestN, best := 0, 0, 0
	for srows.Next() {
		var sector, n int
		if err := srows.Scan(&sector, &n); err != nil {
			return Series{}, err
		}
		total += n
		if n > bestN { // strictly greater: the first (lowest) sector keeps a tie
			bestN, best = n, sector
		}
	}
	if err := srows.Err(); err != nil {
		return Series{}, err
	}
	if total > 0 {
		deg := float64(best) * 22.5
		out.Prevailing = &deg
		out.PrevailingShare = float64(bestN) / float64(total)
	}
	return out, nil
}
