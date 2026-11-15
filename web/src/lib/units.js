// Display units are US customary; the API sends canonical units.
const C_TO_F = (c) => (c * 9) / 5 + 32;
const KT_TO_MPH = (kt) => kt * 1.150779;

export const ELEMENTS = {
	temp: { label: 'Temperature', unit: '°F', conv: C_TO_F, digits: 1 },
	dewpoint: { label: 'Dew point', unit: '°F', conv: C_TO_F, digits: 1 },
	humidity: { label: 'Relative humidity', unit: '%', conv: (v) => v, digits: 0 },
	wind: { label: 'Wind speed', unit: 'mph', conv: KT_TO_MPH, digits: 0 },
	gust: { label: 'Wind gust', unit: 'mph', conv: KT_TO_MPH, digits: 0 },
	visibility: { label: 'Visibility', unit: 'mi', conv: (v) => v, digits: 1 },
	ceiling: { label: 'Ceiling', unit: 'ft', conv: (v) => v, digits: 0 },
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
	if (current.gustKt) parts.push(`gusting ${fmt(current.gustKt, 'gust')}`);
	return parts.join(' · ');
}

const WX = {
	RA: 'rain',
	SN: 'snow',
	DZ: 'drizzle',
	TS: 'thunderstorm',
	BR: 'mist',
	FG: 'fog',
	HZ: 'haze',
	SH: 'showers',
	FZ: 'freezing',
	GR: 'hail',
	GS: 'small hail',
	UP: 'unknown precipitation',
	PL: 'ice pellets',
	IC: 'ice crystals',
	SQ: 'squalls'
};

function weatherText(wx) {
	const intensity = wx.startsWith('-') ? 'Light ' : wx.startsWith('+') ? 'Heavy ' : '';
	const words = [];
	for (let i = 0; i < wx.length; i += 2) {
		const code = wx.slice(i, i + 2).replace(/[+-]/, '');
		if (!code.trim()) continue;
		words.push(WX[code] ?? code);
	}
	const text = words.join(' ');
	return intensity + (text || wx);
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
