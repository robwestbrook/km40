// Run with: npm --prefix web test   (Node's built-in runner, no extra dependency)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sentence, presetRange } from './units.js';

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

test('presetRange has no range for custom or unknown presets', () => {
	assert.equal(presetRange('custom', new Date()), null);
	assert.equal(presetRange('', new Date()), null);
});
