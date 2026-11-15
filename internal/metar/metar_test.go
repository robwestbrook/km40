package metar

import (
	"math"
	"testing"
	"time"
)

var now = time.Date(2026, 11, 15, 2, 0, 0, 0, time.UTC)

func TestParseLiveReport(t *testing.T) {
	o, err := Parse("KM40 150115Z AUTO 00000KT 10SM CLR 04/00 A3028 RMK AO2", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.Station != "KM40" {
		t.Errorf("station = %q", o.Station)
	}
	if want := time.Date(2026, 11, 15, 1, 15, 0, 0, time.UTC); !o.Time.Equal(want) {
		t.Errorf("time = %v, want %v", o.Time, want)
	}
	if o.TempC == nil || *o.TempC != 4 || o.DewpointC == nil || *o.DewpointC != 0 {
		t.Errorf("temp/dew = %v/%v", o.TempC, o.DewpointC)
	}
	if o.WindKt == nil || *o.WindKt != 0 || o.WindDir != nil {
		t.Errorf("wind = %v dir %v", o.WindKt, o.WindDir)
	}
	if o.VisSM == nil || *o.VisSM != 10 {
		t.Errorf("vis = %v", o.VisSM)
	}
	if o.AltimeterInHg == nil || math.Abs(*o.AltimeterInHg-30.28) > 0.001 {
		t.Errorf("altimeter = %v", o.AltimeterInHg)
	}
	if o.Sky != "CLR" {
		t.Errorf("sky = %q", o.Sky)
	}
}

func TestParseWind(t *testing.T) {
	o, _ := Parse("KM40 150115Z 21015G25KT 10SM CLR 04/00 A3028", now)
	if o.WindDir == nil || *o.WindDir != 210 || *o.WindKt != 15 || *o.GustKt != 25 {
		t.Fatalf("gust wind parsed as dir=%v spd=%v gust=%v", o.WindDir, o.WindKt, o.GustKt)
	}
	o, _ = Parse("KM40 150115Z VRB03KT 10SM CLR 04/00 A3028", now)
	if o.WindDir != nil || *o.WindKt != 3 {
		t.Fatalf("variable wind parsed as dir=%v spd=%v", o.WindDir, o.WindKt)
	}
}

func TestParseWeatherAndCeiling(t *testing.T) {
	o, _ := Parse("KM40 150115Z 21010KT 2SM -RA BR SCT020 BKN008 OVC015 14/13 A2992", now)
	if o.Wx != "-RA BR" {
		t.Errorf("wx = %q", o.Wx)
	}
	if o.CeilingFt == nil || *o.CeilingFt != 800 {
		t.Errorf("ceiling = %v", o.CeilingFt)
	}
	if o.Sky != "SCT020 BKN008 OVC015" {
		t.Errorf("sky = %q", o.Sky)
	}
	if *o.VisSM != 2 {
		t.Errorf("vis = %v", o.VisSM)
	}
}

func TestParseFractionalVisibility(t *testing.T) {
	for raw, want := range map[string]float64{
		"KM40 150115Z 00000KT 1/2SM CLR 04/00 A3028":   0.5,
		"KM40 150115Z 00000KT M1/4SM CLR 04/00 A3028":  0.25,
		"KM40 150115Z 00000KT 1 1/2SM CLR 04/00 A3028": 1.5,
		"KM40 150115Z 00000KT P6SM CLR 04/00 A3028":    6,
	} {
		o, _ := Parse(raw, now)
		if o.VisSM == nil || *o.VisSM != want {
			t.Errorf("%q: vis = %v, want %v", raw, o.VisSM, want)
		}
	}
}

// A zero denominator is malformed; it must not store an infinite visibility.
func TestParseZeroDenominatorVisibility(t *testing.T) {
	for _, raw := range []string{
		"KM40 150115Z 00000KT 1/0SM CLR 04/00 A3028",
		"KM40 150115Z 00000KT M1/0SM CLR 04/00 A3028",
	} {
		o, _ := Parse(raw, now)
		if o.VisSM != nil {
			t.Errorf("%q: vis = %v, want unset", raw, *o.VisSM)
		}
	}
}

func TestParseMissingElements(t *testing.T) {
	o, err := Parse("KM40 150115Z AUTO 00000KT 10SM CLR 04/ A3028", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.DewpointC != nil {
		t.Errorf("dewpoint should be nil, got %v", *o.DewpointC)
	}
	o, err = Parse("KM40 150115Z AUTO 00000KT /////KT //// CLR ///// A3028", now)
	if err != nil {
		t.Fatalf("partial report must still parse: %v", err)
	}
	if o.TempC != nil {
		t.Errorf("temp should be nil, got %v", *o.TempC)
	}
}

func TestParseAltimeterQNH(t *testing.T) {
	o, _ := Parse("KM40 150115Z 00000KT 9999 CLR 04/00 Q1013", now)
	if o.AltimeterInHg == nil || math.Abs(*o.AltimeterInHg-29.91) > 0.02 {
		t.Errorf("altimeter = %v", o.AltimeterInHg)
	}
}

func TestParseRemarks(t *testing.T) {
	o, _ := Parse("KM40 150115Z AUTO 00000KT 10SM CLR 04/00 A3028 RMK AO2 P0013 T00441002", now)
	if o.PrecipIn == nil || math.Abs(*o.PrecipIn-0.13) > 0.0001 {
		t.Errorf("precip = %v", o.PrecipIn)
	}
	if *o.TempC != 4.4 || *o.DewpointC != -0.2 {
		t.Errorf("T group temp/dew = %v/%v", *o.TempC, *o.DewpointC)
	}
}

func TestParseMonthRollover(t *testing.T) {
	// Report issued 1 Dec 00:05Z, parsed at 2 Dec 02:00Z (its own month).
	o, _ := Parse("KM40 010005Z AUTO 00000KT 10SM CLR 04/00 A3028", time.Date(2026, 12, 2, 2, 0, 0, 0, time.UTC))
	if want := time.Date(2026, 12, 1, 0, 5, 0, 0, time.UTC); !o.Time.Equal(want) {
		t.Errorf("time = %v, want %v", o.Time, want)
	}
	// A late-month report seen minutes after midnight on the 1st belongs to the prior month.
	o, _ = Parse("KM40 312355Z AUTO 00000KT 10SM CLR 04/00 A3028", time.Date(2026, 11, 1, 0, 10, 0, 0, time.UTC))
	if want := time.Date(2026, 10, 31, 23, 55, 0, 0, time.UTC); !o.Time.Equal(want) {
		t.Errorf("time = %v, want %v", o.Time, want)
	}
}

func TestParseErrorOnNoTime(t *testing.T) {
	if _, err := Parse("not a metar", now); err == nil {
		t.Fatal("expected error")
	}
}

func TestRelativeHumidity(t *testing.T) {
	got := RelativeHumidity(20, 20)
	if math.Abs(got-100) > 0.5 {
		t.Errorf("rh = %v", got)
	}
	got = RelativeHumidity(30, 10)
	if got < 25 || got > 35 {
		t.Errorf("rh = %v, want about 30", got)
	}
}

// Sky cover is the fullest layer, as a percentage of the sky (oktas midpoints).
func TestSkyCoverPercent(t *testing.T) {
	cases := []struct {
		sky  string
		want float64
		ok   bool
	}{
		{"CLR", 0, true},
		{"SKC", 0, true},
		{"NSC", 0, true},
		{"CAVOK", 0, true},
		{"FEW017", 19, true},
		{"FEW017 SCT022", 44, true},
		{"FEW017 SCT022 BKN048", 75, true},
		{"BKN048 OVC060", 100, true},
		{"VV005", 100, true},
		{"SCT040CB", 44, true},
		{"", 0, false},
		{"garbage", 0, false},
	}
	for _, c := range cases {
		got, ok := SkyCoverPercent(c.sky)
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("SkyCoverPercent(%q) = %v, %v; want %v, %v", c.sky, got, ok, c.want, c.ok)
		}
	}
}

func TestParseOmittedWindIsCalm(t *testing.T) {
	o, err := Parse("KM40 150115Z AUTO 10SM CLR 06/00 A3029 RMK AO2", now)
	if err != nil {
		t.Fatal(err)
	}
	if o.WindKt == nil || *o.WindKt != 0 || o.WindDir != nil {
		t.Errorf("omitted wind parsed as %v", o.WindKt)
	}
	o, _ = Parse("KM40 150115Z AUTO /////KT 10SM CLR 06/00 A3029", now)
	if o.WindKt != nil {
		t.Errorf("masked wind should stay unknown, got %v", *o.WindKt)
	}
}
