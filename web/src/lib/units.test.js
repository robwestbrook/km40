// Run with: npm --prefix web test   (Node's built-in runner, no extra dependency)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sentence, presetRange, currentValue, isCircular, bearingText, prevailingText, rangeOf, needleAngle, speedText, ELEMENTS, ARCHIVE_START, ARCHIVE_SINCE } from './units.js';

const obs = (extra) => ({ tempC: 20, windKt: 0, ...extra });
const lead = (weather) => sentence(obs({ weather })).split(' · ')[0];

test('weather intensity prefixes decode instead of being split into letters', () => {
	assert.equal(lead('-RA'), 'Light rain');
	assert.equal(lead('+RA'), 'Heavy rain');
	assert.equal(lead('RA'), 'Rain');
});

test('showers, thunderstorms and freezing precipitation read as phrases', () => {
	assert.equal(lead('+SHRA'), 'Heavy rain showers');
	assert.equal(lead('-SHSN'), 'Light snow showers');
	assert.equal(lead('TSRA'), 'Thunderstorm with rain');
	assert.equal(lead('TS'), 'Thunderstorm');
	assert.equal(lead('FZRA'), 'Freezing rain');
	assert.equal(lead('BLSN'), 'Blowing snow');
	assert.equal(lead('RASN'), 'Rain and snow');
});

test('several weather groups are listed separately', () => {
	assert.equal(lead('-RA BR'), 'Light rain, mist');
	assert.equal(lead('+TSRA FG'), 'Heavy thunderstorm with rain, fog');
});

test('vicinity groups say so', () => {
	assert.equal(lead('VCSH'), 'Showers in the vicinity');
});

test('an unrecognised group is shown as reported, never dropped', () => {
	assert.equal(lead('XYZ'), 'XYZ');
});

test('gusts carry a unit', () => {
	const s = sentence(obs({ windKt: 20, windDir: 180, gustKt: 27 }));
	assert.match(s, /gusting 31 mph/);
});

// A preset is relative to "now". The auto-refresh recomputes it each time, so
// the window must follow the clock instead of staying where it was clicked.
test('presetRange is relative to the time it is asked for', () => {
	const t1 = new Date('2026-11-15T12:00:00Z');
	const t2 = new Date('2026-11-15T12:05:00Z');
	const a = presetRange('24h', t1);
	const b = presetRange('24h', t2);
	assert.equal(a.to, t1.toISOString());
	assert.equal(a.from, '2026-11-14T12:00:00.000Z');
	assert.equal(b.to, t2.toISOString());
	assert.notEqual(a.to, b.to);
	assert.equal(presetRange('7d', t1).from, '2026-11-08T12:00:00.000Z');
	assert.equal(presetRange('30d', t1).from, '2026-10-16T12:00:00.000Z');
	assert.equal(presetRange('all', t1).from, '2012-01-11T00:00:00.000Z');
});

// The footers derive their text from the archive start instead of repeating it.
test('the archive start reads as a date in the page footers', () => {
	assert.equal(ARCHIVE_START, '2012-01-11');
	assert.equal(ARCHIVE_SINCE, '11 Jan 2012');
});

// One place maps an element key to its current value, for both pages.
test('currentValue picks the current reading for each element', () => {
	const c = { tempC: 20, windKt: 7, ceilingFt: 4800, skyCoverPct: 75, precipIn: null };
	assert.equal(currentValue(c, 'temp'), 20);
	assert.equal(currentValue(c, 'wind'), 7);
	assert.equal(currentValue(c, 'ceiling'), 4800);
	assert.equal(currentValue(c, 'sky'), 75);
	assert.equal(currentValue({ windDir: 245 }, 'winddir'), 245);
	assert.equal(currentValue(c, 'precip'), null);
	assert.equal(currentValue(c, 'nope'), null);
	assert.equal(currentValue(null, 'temp'), null);
});

test('every element the server reports has a display definition', () => {
	for (const key of ['temp', 'dewpoint', 'humidity', 'wind', 'gust', 'winddir', 'visibility', 'ceiling', 'sky', 'altimeter', 'precip']) {
		assert.ok(ELEMENTS[key], `missing ELEMENTS.${key}`);
	}
});

// A bearing is circular: no high/low/records, shown as a compass point.
test('only wind direction is circular', () => {
	assert.equal(isCircular('winddir'), true);
	assert.equal(isCircular('wind'), false);
	assert.equal(isCircular('temp'), false);
	assert.equal(isCircular('nope'), false);
});

// A percentage that is always 0 must not autoscale to -1..1 around a flat line.
test('sky cover has a fixed 0-100 axis; other elements autoscale', () => {
	assert.deepEqual(rangeOf('sky'), [0, 100]);
	assert.equal(rangeOf('temp'), null);
	assert.equal(rangeOf('nope'), null);
});

test('prevailing direction reads as compass point and share of reports', () => {
	assert.equal(prevailingText(202.5, 0.14), 'SSW · 14%');
	assert.equal(prevailingText(0, 1), 'N · 100%');
	assert.equal(prevailingText(null, 0), '—');
	assert.equal(prevailingText(undefined, 0), '—');
});

test('a bearing reads as compass point and degrees', () => {
	assert.equal(bearingText(245), 'WSW 245°');
	assert.equal(bearingText(0), 'N 0°');
	assert.equal(bearingText(359.6), 'N 0°'); // rounds to north, never "360"
	assert.equal(bearingText(null), '—');
	assert.equal(bearingText(undefined), '—');
});

test('presetRange has no range for custom or unknown presets', () => {
	assert.equal(presetRange('custom', new Date()), null);
	assert.equal(presetRange('', new Date()), null);
});

test('needleAngle gives the bearing in 0-359, and null when no needle should show', () => {
	assert.equal(needleAngle(245, 9), 245);
	assert.equal(needleAngle(0, 5), 0);
	assert.equal(needleAngle(360, 5), 0); // north is never 360
	assert.equal(needleAngle(-10, 5), 350);
});

test('needleAngle is null for calm, variable and missing data', () => {
	assert.equal(needleAngle(0, 0), null); // calm reported as 00000KT
	assert.equal(needleAngle(245, 0), null);
	assert.equal(needleAngle(null, 6), null); // variable
	assert.equal(needleAngle(undefined, 6), null);
	assert.equal(needleAngle(Number.NaN, 6), null);
	assert.equal(needleAngle(null, null), null);
});

test('speedText reads as mph, and as a dash with no reading', () => {
	assert.equal(speedText(8), '9 mph'); // 8 kt = 9.2 mph
	assert.equal(speedText(0), '0 mph');
	assert.equal(speedText(null), '—');
	assert.equal(speedText(undefined), '—');
	assert.equal(speedText(Number.NaN), '—');
});
