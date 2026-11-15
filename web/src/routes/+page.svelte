<script>
	import { onMount } from 'svelte';
	import Chart from '$lib/Chart.svelte';
	import Window from '$lib/Window.svelte';
	import { getJSON } from '$lib/api.js';
	import { sentence, fmt, unit, localTime, clockTime, compass, ELEMENTS, windowHours, presetRange, currentValue, isCircular, prevailingText } from '$lib/units.js';

	let summary = null;
	let error = '';
	let from = '';
	let to = '';
	let points = 96;
	let loading = false;

	$: span = windowHours(from, to);

	// Read the window directly: the derived `span` is stale until the next
	// update, and load() runs right after from/to change.
	function budget() {
		return Math.min(600, Math.max(48, Math.round(windowHours(from, to) * 2)));
	}

	async function load() {
		loading = true;
		error = '';
		try {
			const q = new URLSearchParams();
			if (from) q.set('from', from);
			if (to) q.set('to', to);
			q.set('points', String(budget()));
			summary = await getJSON(`/api/summary?${q}`);
			points = budget();
		} catch (e) {
			error = String(e.message ?? e);
		} finally {
			loading = false;
		}
	}

	let preset = '24h';

	function setWindow(f, t, p) {
		from = f;
		to = t;
		preset = p;
		load();
	}

	// A preset is relative to now, so the auto-refresh moves it forward; a
	// custom range stays exactly as chosen.
	function refresh() {
		const range = presetRange(preset, new Date());
		if (range) {
			from = range.from;
			to = range.to;
		}
		return load();
	}

	onMount(() => {
		load();
		const timer = setInterval(refresh, 5 * 60 * 1000);
		return () => clearInterval(timer);
	});

	function current(key) {
		return currentValue(summary?.current, key);
	}

	function subline(key) {
		const c = summary?.current;
		if (key === 'wind' && c?.windKt) {
			return `${compass(c.windDir)}${c.gustKt ? ` · gust ${fmt(c.gustKt, 'gust')} ${unit('gust')}` : ''}`;
		}
		if (key === 'winddir') {
			if (c?.windDir != null) return compass(c.windDir);
			if (c?.windKt === 0) return 'calm';
			return c?.windKt > 0 ? 'variable' : ''; // wind with no direction is variable
		}
		if (key === 'temp' && c?.humidity != null) return `${fmt(c.humidity, 'humidity')}% humidity`;
		if (key === 'precip' && c?.precipIn == null) return 'none in the last report';
		if (key === 'sky' && c?.sky) return c.sky;
		return '';
	}
</script>

<svelte:head>
	<title>KM40 · Aberdeen weather</title>
</svelte:head>

<header class="mb-8 text-center">
	<p class="text-[11px] font-semibold uppercase tracking-[0.2em] text-sky-400/80">
		Metar KM40 · Aberdeen / Monroe County Airport
	</p>
	<h1 class="mt-2 text-2xl font-semibold text-slate-100 sm:text-3xl">
		{#if summary}
			{sentence(summary.current)}
		{:else if error}
			<span class="text-rose-400">Cannot reach the server</span>
		{:else}
			<span class="text-slate-500">Loading…</span>
		{/if}
	</h1>
	<p class="mt-2 text-sm text-slate-400">
		{#if summary?.observedAt}
			Observed {clockTime(summary.observedAt)} local ({summary.ageMinutes} min ago)
			{#if summary.current.altimeterInHg}
				· {fmt(summary.current.altimeterInHg, 'altimeter')} {unit('altimeter')}
			{/if}
			{#if summary.current.visibilitySM != null}
				· {fmt(summary.current.visibilitySM, 'visibility')} {unit('visibility')} visibility
			{/if}
		{:else}
			No observations yet — the first backfill may still be running.
		{/if}
	</p>
	{#if error}
		<p class="mt-2 rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-300">{error}</p>
	{/if}
	<div class="mt-4 flex items-center justify-center gap-3">
		<button
			type="button"
			class="rounded-full bg-slate-800/60 px-3 py-1 text-xs text-slate-400 hover:text-slate-200"
			on:click={load}
			disabled={loading}
		>
			{loading ? 'Refreshing…' : 'Refresh'}
		</button>
	</div>
	<div class="mt-4">
		<Window {from} {to} onchange={setWindow} />
	</div>
</header>

{#if summary}
	<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each summary.elements as el (el.element)}
			<section
				class="flex flex-col rounded-xl border border-slate-800/80 bg-slate-900/50 p-4 backdrop-blur transition hover:border-slate-700"
			>
				<div class="flex items-start justify-between gap-2">
					<div>
						<h2 class="text-sm font-medium text-slate-300">{ELEMENTS[el.element]?.label ?? el.element}</h2>
						<p class="mt-1 text-2xl font-semibold text-slate-50">
							{fmt(current(el.element), el.element)}
							<span class="text-sm font-normal text-slate-500">{unit(el.element)}</span>
						</p>
						<p class="h-4 text-xs text-slate-500">{subline(el.element)}</p>
					</div>
					<a
						href={`/element/${el.element}`}
						class="shrink-0 rounded-full border border-slate-700 px-2.5 py-1 text-[11px] text-slate-400 hover:border-sky-400/50 hover:text-sky-300"
					>
						Details →
					</a>
				</div>

				<div class="mt-3">
					<Chart points={el.points} name={el.element} rangeHours={span} height={78} />
				</div>

				<dl class="mt-2 grid grid-cols-2 gap-x-3 gap-y-1 border-t border-slate-800/80 pt-2 text-[11px]">
					{#if isCircular(el.element)}
						<!-- A bearing has a prevailing direction, not a high, low or records. -->
						<div class="col-span-2 flex justify-between">
							<dt class="text-slate-500">Prevailing</dt>
							<dd class="text-slate-300">{prevailingText(el.prevailing, el.prevailingShare)}</dd>
						</div>
					{:else}
						<div class="flex justify-between">
							<dt class="text-slate-500">Low</dt>
							<dd class="text-slate-300">{el.hasData ? fmt(el.min, el.element) : '—'}</dd>
						</div>
						<div class="flex justify-between">
							<dt class="text-slate-500">High</dt>
							<dd class="text-slate-300">{el.hasData ? fmt(el.max, el.element) : '—'}</dd>
						</div>
						<div class="flex justify-between">
							<dt class="text-slate-500">Record high</dt>
							<dd class="text-slate-300" title={el.recordHighDate}>
								{el.hasRecord ? `${fmt(el.recordHigh, el.element)} · ${el.recordHighDate.slice(0, 4)}` : '—'}
							</dd>
						</div>
						<div class="flex justify-between">
							<dt class="text-slate-500">Record low</dt>
							<dd class="text-slate-300" title={el.recordLowDate}>
								{el.hasRecord ? `${fmt(el.recordLow, el.element)} · ${el.recordLowDate.slice(0, 4)}` : '—'}
							</dd>
						</div>
					{/if}
				</dl>
			</section>
		{/each}
	</div>

	{#if summary.raw}
		<p class="mt-8 overflow-x-auto rounded-lg bg-slate-900/40 px-3 py-2 text-center font-mono text-[11px] text-slate-500">
			{summary.raw}
		</p>
	{/if}
{/if}
