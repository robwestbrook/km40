// Run with: npm --prefix web test   (Node's built-in runner, no extra dependency)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { sentence } from './units.js';

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
