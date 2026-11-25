<script>
	// A compass dial: the needle points to where the wind comes from, and the
	// speed reads in the lower third. `bearing` is from needleAngle() (null =
	// no needle: calm, variable or no data); `speed` is from speedText().
	export let bearing = null;
	export let speed = '—';
	export let size = 190;
</script>

<svg
	viewBox="0 0 200 200"
	width={size}
	height={size}
	role="img"
	aria-label={bearing === null
		? speed === '—'
			? 'Wind direction unavailable'
			: `Wind direction not available, ${speed}`
		: `Wind from ${Math.round(bearing)} degrees, ${speed}`}
>
	<circle cx="100" cy="100" r="82" fill="#0b1220" stroke="#334155" stroke-width="2" />
	<g stroke="#475569" stroke-width="1.5">
		<line x1="100" y1="18" x2="100" y2="30" />
		<line x1="182" y1="100" x2="170" y2="100" />
		<line x1="100" y1="182" x2="100" y2="170" />
		<line x1="18" y1="100" x2="30" y2="100" />
	</g>
	<g stroke="#334155" stroke-width="1">
		<line x1="100" y1="18" x2="100" y2="24" transform="rotate(45 100 100)" />
		<line x1="100" y1="18" x2="100" y2="24" transform="rotate(135 100 100)" />
		<line x1="100" y1="18" x2="100" y2="24" transform="rotate(225 100 100)" />
		<line x1="100" y1="18" x2="100" y2="24" transform="rotate(315 100 100)" />
	</g>
	<g font-size="13" font-weight="600" fill="#94a3b8" text-anchor="middle" font-family="sans-serif">
		<text x="100" y="46" fill="#38bdf8">N</text>
		<text x="156" y="105">E</text>
		<text x="100" y="165">S</text>
		<text x="44" y="105">W</text>
	</g>
	{#if bearing !== null}
		<g transform="rotate({bearing} 100 100)">
			<polygon points="100,34 106,100 94,100" fill="#38bdf8" />
			<polygon points="94,100 106,100 100,122" fill="#475569" />
		</g>
	{/if}
	<circle cx="100" cy="100" r="5" fill="#0f172a" stroke="#38bdf8" stroke-width="2" />
	<text x="100" y="152" font-size="20" font-weight="600" fill="#f1f5f9" text-anchor="middle" font-family="sans-serif">{speed}</text>
</svg>
