// Display units are US customary; the API sends canonical units.
const C_TO_F = (c) => (c * 9) / 5 + 32;
const KT_TO_MPH = (kt) => kt * 1.150779;

export const ELEMENTS = {
	temp: { label: 'Temperature', unit: '°F', conv: C_TO_F, digits: 1 },
	dewpoint: { label: 'Dew point', unit: '°F', conv: C_TO_F, digits: 1 },
	humidity: { label: 'Relative humidity', unit: '%', conv: (v) => v, digits: 0 },
	wind: { label: 'Wind speed', unit: 'mph', conv: KT_TO_MPH, digits: 0 },
	gust: { label: 'Wind gust', unit: 'mph', conv: KT_TO_MPH, digits: 0 },
	// A bearing is circular: it has a prevailing direction, not a high or low,
	// and no records.
	winddir: { label: 'Wind direction', unit: '°', conv: (v) => v, digits: 0, circular: true },
	visibility: { label: 'Visibility', unit: 'mi', conv: (v) => v, digits: 1 },
	ceiling: { label: 'Ceiling', unit: 'ft', conv: (v) => v, digits: 0 },
	sky: { label: 'Sky cover', unit: '%', conv: (v) => v, digits: 0, range: [0, 100] },
	altimeter: { label: 'Altimeter', unit: 'inHg', conv: (v) => v, digits: 2 },
	precip: { label: 'Precipitation', unit: 'in', conv: (v) => v, digits: 2 }
};

export function fmt(value, key) {
	if (value === null || value === undefined || Number.isNaN(value)) return '—';
	const def = ELEMENTS[key] ?? { conv: (v) => v, digits: 1 };
	const v = def.conv(value);
	return v.toLocaleString('en-US', {
		minimumFractionDigits: def.digits,
		maximumFractionDigits: def.digits
	});
}

// The current reading for an element key, from the API's `current` object.
// One map for the dashboard and the element page.
export function currentValue(c, key) {
	if (!c) return null;
	return (
		{
			temp: c.tempC,
			dewpoint: c.dewpointC,
			humidity: c.humidity,
			wind: c.windKt,
			gust: c.gustKt,
			winddir: c.windDir,
			visibility: c.visibilitySM,
			ceiling: c.ceilingFt,
			sky: c.skyCoverPct,
			altimeter: c.altimeterInHg,
			precip: c.precipIn
		}[key] ?? null
	);
}

export function unit(key) {
	return ELEMENTS[key]?.unit ?? '';
}

export function convert(value, key) {
	return ELEMENTS[key]?.conv(value) ?? value;
}

const DIRS = ['N', 'NNE', 'NE', 'ENE', 'E', 'ESE', 'SE', 'SSE', 'S', 'SSW', 'SW', 'WSW', 'W', 'WNW', 'NW', 'NNW'];

export function compass(deg) {
	if (deg === null || deg === undefined) return '';
	return DIRS[Math.round(deg / 22.5) % 16];
}

// A fixed chart axis for elements with natural bounds, else null (autoscale).
export function rangeOf(key) {
	return ELEMENTS[key]?.range ?? null;
}

export function isCircular(key) {
	return ELEMENTS[key]?.circular === true;
}

// "SSW · 14%": the compass point a wind blows from most often, and the share
// of reports it accounts for. `deg` is the compass point's centre.
export function prevailingText(deg, share) {
	if (deg === null || deg === undefined || Number.isNaN(deg)) return '—';
	return `${compass(deg)} · ${Math.round(share * 100)}%`;
}

// "WSW 245°" for a bearing in degrees; never "360°".
export function bearingText(deg) {
	if (deg === null || deg === undefined || Number.isNaN(deg)) return '—';
	return `${compass(deg)} ${Math.round(deg) % 360}°`;
}

export function localTime(iso) {
	if (!iso) return '—';
	return new Date(iso).toLocaleString('en-US', {
		timeZone: 'America/Chicago',
		month: 'short',
		day: 'numeric',
		hour: 'numeric',
		minute: '2-digit'
	});
}

// The window for a preset button, relative to `now`; null for a custom range
// or anything unrecognised. Kept pure so a refresh can recompute it as time
// passes instead of reusing the instants from the moment of the click.
export function presetRange(preset, now) {
	const start = new Date(now);
	if (preset === '24h') start.setTime(now.getTime() - 24 * 3600e3);
	else if (preset === '7d') start.setTime(now.getTime() - 7 * 86400e3);
	else if (preset === '30d') start.setTime(now.getTime() - 30 * 86400e3);
	else if (preset === '1y') start.setDate(now.getDate() - 365);
	else if (preset === 'all') start.setTime(Date.UTC(2012, 0, 11));
	else return null;
	return { from: start.toISOString(), to: now.toISOString() };
}

const DATE_ONLY = /^\d{4}-\d{2}-\d{2}$/;

// Hours covered by a window. A date-only end is a whole station-local day, so
// it runs to the end of that day, matching the API.
export function windowHours(from, to) {
	if (!from || !to) return 24;
	const end = new Date(to).getTime() + (DATE_ONLY.test(to) ? 24 * 3600e3 : 0);
	return (end - new Date(from).getTime()) / 3600e3;
}

// A date-only string is a calendar day, not an instant. Format its parts so a
// time-zone conversion can never move it to the neighbouring day.
export function calendarDay(s) {
	if (!s) return '';
	if (DATE_ONLY.test(s)) {
		const [y, m, d] = s.split('-');
		return `${+m}/${+d}/${y}`;
	}
	return new Date(s).toLocaleDateString('en-US', { timeZone: 'America/Chicago' });
}

export function clockTime(iso) {
	if (!iso) return '—';
	return new Date(iso).toLocaleTimeString('en-US', {
		timeZone: 'America/Chicago',
		hour: 'numeric',
		minute: '2-digit'
	});
}

// Plain-language summary from the latest observation.
export function sentence(current) {
	if (!current || current.tempC === null || current.tempC === undefined) {
		return 'Waiting for the first observation.';
	}
	const parts = [];
	if (current.weather) {
		parts.push(weatherText(current.weather));
	} else {
		parts.push(skyText(current.sky));
	}
	parts.push(`${fmt(current.tempC, 'temp')}${unit('temp')}`);
	if (current.windKt !== null && current.windKt !== undefined) {
		if (current.windKt === 0) {
			parts.push('calm');
		} else {
			const dir = compass(current.windDir);
			parts.push(`${dir ? dir + ' ' : ''}${fmt(current.windKt, 'wind')} ${unit('wind')}`);
		}
	}
	if (current.gustKt) parts.push(`gusting ${fmt(current.gustKt, 'gust')} ${unit('gust')}`);
	return parts.join(' · ');
}

// METAR present-weather codes. Descriptors modify a phenomenon ("blowing snow");
// TS and SH are handled separately because they read as phrases.
const DESCRIPTOR = {
	MI: 'shallow',
	BC: 'patchy',
	PR: 'partial',
	DR: 'low drifting',
	BL: 'blowing',
	FZ: 'freezing'
};

const PHENOMENON = {
	RA: 'rain',
	SN: 'snow',
	DZ: 'drizzle',
	SG: 'snow grains',
	IC: 'ice crystals',
	PL: 'ice pellets',
	GR: 'hail',
	GS: 'small hail',
	UP: 'unknown precipitation',
	BR: 'mist',
	FG: 'fog',
	FU: 'smoke',
	VA: 'volcanic ash',
	DU: 'dust',
	SA: 'sand',
	HZ: 'haze',
	PY: 'spray',
	PO: 'dust whirls',
	SQ: 'squalls',
	FC: 'funnel cloud',
	SS: 'sandstorm',
	DS: 'duststorm'
};

// One group such as "-SHRA", "+TSRA", "VCSH" or "FZFG" -> "light rain showers".
// A group that does not match the grammar is returned unchanged, never dropped.
function weatherGroup(tok) {
	const m = /^([+-])?(VC)?((?:[A-Z]{2})+)$/.exec(tok);
	if (!m) return tok;
	let thunder = false;
	let showers = false;
	const descriptors = [];
	const phenomena = [];
	for (const code of m[3].match(/../g)) {
		if (code === 'TS') thunder = true;
		else if (code === 'SH') showers = true;
		else if (DESCRIPTOR[code]) descriptors.push(DESCRIPTOR[code]);
		else if (PHENOMENON[code]) phenomena.push(PHENOMENON[code]);
		else return tok; // an unknown code: show the group as reported
	}
	let core = phenomena.join(' and ');
	if (showers) core = core ? `${core} showers` : 'showers';
	core = [...descriptors, core].filter(Boolean).join(' ');
	if (thunder) core = core ? `thunderstorm with ${core}` : 'thunderstorm';
	if (!core) return tok;
	const intensity = m[1] === '-' ? 'light ' : m[1] === '+' ? 'heavy ' : '';
	return `${intensity}${core}${m[2] ? ' in the vicinity' : ''}`;
}

function weatherText(wx) {
	const text = wx.trim().split(/\s+/).map(weatherGroup).join(', ');
	return text.charAt(0).toUpperCase() + text.slice(1);
}

function skyText(sky) {
	if (!sky || /^(CLR|SKC|NSC|NCD|CAVOK)/.test(sky)) return 'Clear';
	const layers = sky.trim().split(/\s+/);
	const top = layers[layers.length - 1];
	const m = /^(FEW|SCT|BKN|OVC|VV)(\d{3})/.exec(top);
	if (!m) return 'Cloudy';
	const amount = { FEW: 'A few clouds', SCT: 'Scattered clouds', BKN: 'Broken clouds', OVC: 'Overcast' }[m[1]] ?? 'Cloudy';
	return `${amount} at ${(parseInt(m[2], 10) * 100).toLocaleString()} ft`;
}
