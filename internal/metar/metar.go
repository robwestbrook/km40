// Package metar parses the subset of METAR groups that KM40 reports.
package metar

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Obs is one decoded METAR. Missing elements are nil; the raw report is
// always kept so a parser fix can replay history.
type Obs struct {
	Time          time.Time
	Station       string
	Raw           string
	TempC         *float64
	DewpointC     *float64
	WindDir       *int
	WindKt        *float64
	GustKt        *float64
	VisSM         *float64
	Sky           string
	CeilingFt     *int
	Wx            string
	AltimeterInHg *float64
	PrecipIn      *float64
}

var (
	windRe     = regexp.MustCompile(`^(\d{3}|VRB)(\d{2,3})(?:G(\d{2,3}))?KT$`)
	stationRe  = regexp.MustCompile(`^[A-Z][A-Z0-9]{3}$`)
	skyRe      = regexp.MustCompile(`^(FEW|SCT|BKN|OVC|VV)(\d{3}|///)(CB|TCU)?$`)
	tempRe     = regexp.MustCompile(`^(M?\d{2})/(M?\d{2})?$`)
	altRe      = regexp.MustCompile(`^A(\d{4})$`)
	qnhRe      = regexp.MustCompile(`^Q(\d{3,4})$`)
	fracVisRe  = regexp.MustCompile(`^(\d+)?/(\d+)SM$`)
	wholeVisRe = regexp.MustCompile(`^(\d+)SM$`)
	metersRe   = regexp.MustCompile(`^\d{4}$`)
	wxRe       = regexp.MustCompile(`^[+-]?(?:VC)?(?:MI|PR|BC|DR|BL|SH|TS|FZ|RA|SN|DZ|SG|IC|PL|GR|GS|UP|BR|FG|FU|VA|DU|SA|HZ|PY|PO|SQ|FC|SS|DS)+$`)
	precipRe   = regexp.MustCompile(`^P(\d{4})$`)
	tGroupRe   = regexp.MustCompile(`^T(\d{4})(\d{4})$`)
)

// Parse decodes a raw METAR. now anchors the day-of-month-only timestamp.
func Parse(raw string, now time.Time) (Obs, error) {
	o := Obs{Raw: strings.TrimSpace(raw), Sky: ""}
	fields := strings.Fields(o.Raw)
	if len(fields) == 0 {
		return o, errors.New("metar: empty report")
	}
	i := 0
	if fields[i] == "METAR" || fields[i] == "SPECI" {
		i++
	}
	if i < len(fields) && stationRe.MatchString(fields[i]) {
		o.Station = fields[i]
		i++
	}

	body := fields[i:]
	timeAt := -1
	for j, tok := range body {
		if t, ok := parseTimeToken(tok, now); ok {
			o.Time, timeAt = t, j
			break
		}
	}
	if timeAt < 0 {
		return o, errors.New("metar: no observation time")
	}
	body = body[timeAt+1:]

	var remarks []string
	for j, tok := range body {
		if tok == "RMK" {
			remarks = body[j+1:]
			body = body[:j]
			break
		}
	}

	var sky []string
	windSeen := false
	for j := 0; j < len(body); j++ {
		tok := body[j]
		switch {
		case tok == "AUTO" || tok == "COR" || tok == "NIL":
		case tok == "CAVOK":
			o.VisSM = new(6.2)
			sky = append(sky, "CAVOK")
		case windRe.MatchString(tok):
			windSeen = true
			m := windRe.FindStringSubmatch(tok)
			spd, _ := strconv.Atoi(m[2])
			o.WindKt = new(float64(spd))
			if m[1] != "VRB" && spd > 0 {
				if dir, err := strconv.Atoi(m[1]); err == nil {
					o.WindDir = &dir
				}
			}
			if m[3] != "" {
				g, _ := strconv.Atoi(m[3])
				o.GustKt = new(float64(g))
			}
		case wholeVisRe.MatchString(tok) && !skyRe.MatchString(tok):
			m := wholeVisRe.FindStringSubmatch(tok)
			v, _ := strconv.ParseFloat(m[1], 64)
			o.VisSM = new(v)
		case fracVisRe.MatchString(tok):
			m := fracVisRe.FindStringSubmatch(tok)
			v, ok := fraction(m[1], m[2])
			if !ok {
				break
			}
			if j > 0 {
				if whole, err := strconv.ParseFloat(body[j-1], 64); err == nil && whole < 10 {
					v += whole // "1 1/2SM"
				}
			}
			o.VisSM = new(v)
		case strings.HasPrefix(tok, "M") && fracVisRe.MatchString(tok[1:]):
			m := fracVisRe.FindStringSubmatch(tok[1:])
			if v, ok := fraction(m[1], m[2]); ok {
				o.VisSM = new(v)
			}
		case strings.HasPrefix(tok, "P") && wholeVisRe.MatchString(tok[1:]):
			m := wholeVisRe.FindStringSubmatch(tok[1:])
			v, _ := strconv.ParseFloat(m[1], 64)
			o.VisSM = new(v)
		case metersRe.MatchString(tok):
			// 4-digit metres visibility (ICAO style), e.g. 9999.
			if m, err := strconv.Atoi(tok); err == nil && m > 0 {
				o.VisSM = new(float64(m) / 1609.344)
			}
		case skyRe.MatchString(tok):
			sky = append(sky, tok)
			m := skyRe.FindStringSubmatch(tok)
			if m[2] != "///" && (m[1] == "BKN" || m[1] == "OVC" || m[1] == "VV") {
				base, _ := strconv.Atoi(m[2])
				ft := base * 100
				if o.CeilingFt == nil || ft < *o.CeilingFt {
					o.CeilingFt = &ft
				}
			}
		case tok == "SKC" || tok == "CLR" || tok == "NSC" || tok == "NCD":
			sky = append(sky, tok)
		case tempRe.MatchString(tok):
			m := tempRe.FindStringSubmatch(tok)
			o.TempC = new(signed(m[1]))
			if m[2] != "" {
				o.DewpointC = new(signed(m[2]))
			}
		case altRe.MatchString(tok):
			m := altRe.FindStringSubmatch(tok)
			v, _ := strconv.ParseFloat(m[1], 64)
			o.AltimeterInHg = new(v / 100)
		case qnhRe.MatchString(tok):
			m := qnhRe.FindStringSubmatch(tok)
			v, _ := strconv.ParseFloat(m[1], 64)
			o.AltimeterInHg = new(v * 0.0295299830714)
		case wxRe.MatchString(tok):
			o.Wx = strings.TrimSpace(o.Wx + " " + tok)
		}
	}
	for _, tok := range body {
		if strings.HasSuffix(tok, "KT") {
			windSeen = true // includes masked groups such as /////KT
		}
	}
	if !windSeen {
		// An omitted wind group means calm; a masked group stays unknown.
		o.WindKt = new(0.0)
	}
	o.Sky = strings.Join(sky, " ")

	for _, tok := range remarks {
		if m := precipRe.FindStringSubmatch(tok); m != nil {
			v, _ := strconv.ParseFloat(m[1], 64)
			o.PrecipIn = new(v / 100)
		}
		if m := tGroupRe.FindStringSubmatch(tok); m != nil {
			// Tenths of a degree; more precise than the whole-degree group.
			o.TempC = new(tenths(m[1]))
			o.DewpointC = new(tenths(m[2]))
		}
	}
	return o, nil
}

// skyCoverPct is the share of sky each cover amount stands for: the midpoint
// of its oktas range (FEW 1-2, SCT 3-4, BKN 5-7, OVC 8).
var skyCoverPct = map[string]float64{
	"CLR": 0, "SKC": 0, "NSC": 0, "NCD": 0, "CAVOK": 0,
	"FEW": 19, "SCT": 44, "BKN": 75, "OVC": 100, "VV": 100,
}

// SkyCoverSQL returns a SQL expression that computes the same value as
// SkyCoverPercent over the text column col (NULL when there is no layer). It is
// built from the same table so the two cannot drift apart. It is plain SQL on
// purpose: a Go function called per row made a 14-year aggregate several times
// slower.
func SkyCoverSQL(col string) string {
	byPct := map[float64][]string{}
	for amount, pct := range skyCoverPct {
		byPct[pct] = append(byPct[pct], amount)
	}
	pcts := make([]float64, 0, len(byPct))
	for pct := range byPct {
		pcts = append(pcts, pct)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(pcts))) // fullest first
	var b strings.Builder
	b.WriteString("CASE")
	for _, pct := range pcts {
		amounts := byPct[pct]
		sort.Strings(amounts)
		conds := make([]string, len(amounts))
		for i, a := range amounts {
			conds[i] = fmt.Sprintf("%s LIKE '%%%s%%'", col, a)
		}
		fmt.Fprintf(&b, " WHEN %s THEN %g", strings.Join(conds, " OR "), pct)
	}
	b.WriteString(" END")
	return b.String()
}

// SkyCoverPercent reduces a stored sky string such as "FEW017 BKN048" to the
// cover of its fullest layer, as a percentage. ok is false when the string has
// no recognisable layer, so unknown reports stay out of statistics.
func SkyCoverPercent(sky string) (pct float64, ok bool) {
	for _, tok := range strings.Fields(sky) {
		amount := tok
		if m := skyRe.FindStringSubmatch(tok); m != nil {
			amount = m[1]
		}
		if v, known := skyCoverPct[amount]; known && (!ok || v > pct) {
			pct, ok = v, true
		}
	}
	return pct, ok
}

func signed(s string) float64 {
	neg := strings.HasPrefix(s, "M")
	s = strings.TrimPrefix(s, "M")
	v, _ := strconv.ParseFloat(s, 64)
	if neg {
		return -v
	}
	return v
}

// tenths decodes a T-group value: tenths of a degree, where a leading 1 means
// negative (1002 is -0.2, 0044 is 4.4).
func tenths(s string) float64 {
	v, _ := strconv.Atoi(s)
	if s[0] == '1' {
		v = -(v - 1000)
	}
	return float64(v) / 10
}

// fraction reads a visibility fraction such as "1/4". The numerator group is
// optional in the pattern (empty counts as 0); ok is false for a zero
// denominator, which would otherwise store an infinite visibility.
func fraction(num, den string) (v float64, ok bool) {
	n := 0.0
	if num != "" {
		n, _ = strconv.ParseFloat(num, 64)
	}
	d, _ := strconv.ParseFloat(den, 64)
	if d == 0 {
		return 0, false
	}
	return n / d, true
}

func parseTimeToken(tok string, now time.Time) (time.Time, bool) {
	if len(tok) != 7 || tok[6] != 'Z' {
		return time.Time{}, false
	}
	day, err1 := strconv.Atoi(tok[0:2])
	hour, err2 := strconv.Atoi(tok[2:4])
	min, err3 := strconv.Atoi(tok[4:6])
	if err1 != nil || err2 != nil || err3 != nil || day < 1 || day > 31 || hour > 23 || min > 59 {
		return time.Time{}, false
	}
	now = now.UTC()
	var best time.Time
	bestDiff := time.Duration(math.MaxInt64)
	for _, month := range []time.Month{now.Month() - 1, now.Month(), now.Month() + 1} {
		c := time.Date(now.Year(), month, day, hour, min, 0, 0, time.UTC)
		if c.Day() != day {
			continue // day rolled over: invalid for that month
		}
		diff := c.Sub(now)
		if diff < 0 {
			diff = -diff
		}
		if diff < bestDiff {
			best, bestDiff = c, diff
		}
	}
	if bestDiff == time.Duration(math.MaxInt64) {
		return time.Time{}, false
	}
	return best, true
}

// RelativeHumidity returns percent relative humidity from Celsius temperatures.
func RelativeHumidity(tempC, dewC float64) float64 {
	es := func(t float64) float64 { return 6.112 * math.Exp(17.67*t/(t+243.5)) }
	rh := 100 * es(dewC) / es(tempC)
	return math.Max(0, math.Min(100, rh))
}
