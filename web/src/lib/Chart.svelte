<script>
	import { onMount } from 'svelte';
	import Chart from 'chart.js/auto';
	import { convert, unit } from './units.js';

	export let points = [];
	export let name = 'temp';
	export let height = 90;
	export let large = false;
	export let rangeHours = 24;

	let canvas;
	let chart;

	const narrow = (iso) =>
		new Date(iso).toLocaleTimeString('en-US', {
			timeZone: 'America/Chicago',
			hour: 'numeric',
			minute: '2-digit'
		});
	const shortDate = (iso) =>
		new Date(iso).toLocaleDateString('en-US', {
			timeZone: 'America/Chicago',
			month: 'short',
			day: 'numeric'
		});
	const monthYear = (iso) =>
		new Date(iso).toLocaleDateString('en-US', {
			timeZone: 'America/Chicago',
			month: 'short',
			year: 'numeric'
		});

	function labels() {
		return points.map((p) =>
			rangeHours <= 72 ? narrow(p.t) : rangeHours <= 24 * 90 ? shortDate(p.t) : monthYear(p.t)
		);
	}

	function config() {
		return {
			type: 'line',
			data: {
				labels: labels(),
				datasets: [
					{
						data: points.map((p) => convert(p.v, name)),
						borderColor: '#38bdf8',
						backgroundColor: 'rgba(56,189,248,0.10)',
						fill: true,
						borderWidth: large ? 2 : 1.5,
						pointRadius: 0,
						pointHitRadius: 10,
						tension: 0.25,
						spanGaps: true
					}
				]
			},
			options: {
				responsive: true,
				maintainAspectRatio: false,
				animation: false,
				interaction: { intersect: false, mode: 'index' },
				plugins: {
					legend: { display: false },
					tooltip: {
						enabled: large,
						displayColors: false,
						callbacks: {
							label: (ctx) => `${ctx.parsed.y.toFixed(large ? 2 : 1)} ${unit(name)}`
						}
					}
				},
				scales: {
					x: {
						grid: { display: false },
						ticks: { color: '#64748b', maxTicksLimit: large ? 8 : 4, maxRotation: 0, font: { size: 10 } }
					},
					y: {
						grid: { color: 'rgba(148,163,184,0.10)' },
						border: { display: false },
						ticks: {
							color: '#64748b',
							maxTicksLimit: large ? 6 : 3,
							font: { size: 10 },
							callback: (v) => `${v}`
						}
					}
				}
			}
		};
	}

	onMount(() => {
		chart = new Chart(canvas, config());
		return () => chart.destroy();
	});

	// Rebuild on data or window change.
	$: if (chart && csig(points, rangeHours, name)) {
		chart.data = config().data;
		chart.options = config().options;
		chart.update('none');
	}
	let last = '';
	function csig(p, r, n) {
		const s = `${n}|${r}|${p.length}|${p.length ? p[p.length - 1].t : ''}`;
		if (s === last) return false;
		last = s;
		return true;
	}
</script>

<div class="relative" style={`height:${height}px`}>
	{#if points.length === 0}
		<div class="absolute inset-0 grid place-items-center text-xs text-slate-500">No data yet</div>
	{/if}
	<canvas bind:this={canvas}></canvas>
</div>
