<script>
	import { page } from '$app/stores';
	import Chart from '$lib/Chart.svelte';
	import Window from '$lib/Window.svelte';
	import { getJSON } from '$lib/api.js';
	import { fmt, unit, localTime, ELEMENTS, compass, windowHours, calendarDay, currentValue, isCircular, prevailingText } from '$lib/units.js';

	let latest = null;
	let stats = null;
	let error = '';
	let from = '';
	let to = '';
	let loading = false;

	$: key = $page.params.key;
	$: meta = ELEMENTS[key] ?? { label: key, unit: '' };

	$: span = windowHours(from, to);

	function budget() {
		// Read the window directly: the derived `span` is stale until the next
		// update, and this runs right after from/to change.
		return Math.min(800, Math.max(120, Math.round(windowHours(from, to) * 2)));
	}

	$: reading = currentValue(latest?.current, key);

	async function load() {
		loading = true;
		error = '';
		try {
			const q = new URLSearchParams();
			if (from) q.set('from', from);
			if (to) q.set('to', to);
			q.set('points', String(budget()));
			[stats, latest] = await Promise.all([
				getJSON(`/api/series?element=${encodeURIComponent(key)}&${q}`),
				getJSON('/api/latest')
			]);
		} catch (e) {
			stats = null;
			error = String(e.message ?? e);
		} finally {
			loading = false;
		}
	}

	function setWindow(f, t) {
		from = f;
		to = t;
		load();
	}

	// Reloads when the route's element changes, including the first render.
	let loadedKey = '';
	$: if (key && key !== loadedKey) {
		loadedKey = key;
		stats = null;
		load();
	}
</script>

<svelte:head>
	<title>{meta.label} · KM40</title>
</svelte:head>

<header class="mb-6">
	<a href="/" class="text-xs text-slate-500 hover:text-sky-300">← All conditions</a>
	<div class="mt-2 flex flex-wrap items-end justify-between gap-3">
		<div>
			<h1 class="text-2xl font-semibold text-slate-100">{meta.label}</h1>
			<p class="text-xs text-slate-500">KM40 · records since 11 Jan 2012 · times local</p>
		</div>
		<div class="text-right">
			<p class="text-3xl font-semibold text-slate-50">
				{fmt(reading, key)}
				<span class="text-base font-normal text-slate-500">{unit(key)}</span>
			</p>
			{#if (key === 'wind' || key === 'winddir') && latest?.current?.windDir != null}
				<p class="text-xs text-slate-500">
					from {compass(latest.current.windDir)} ({latest.current.windDir}°)
				</p>
			{/if}
		</div>
	</div>
</header>

{#if error}
	<p class="mb-4 rounded-lg bg-rose-500/10 px-3 py-2 text-xs text-rose-300">{error}</p>
{/if}

<section class="rounded-xl border border-slate-800/80 bg-slate-900/50 p-4">
	<div class="mb-3 flex items-center justify-between">
		<p class="text-xs text-slate-500">
			{#if from}{calendarDay(from)}{/if}
			{#if from && to}–{calendarDay(to)}{/if}
		</p>
		<button
			type="button"
			class="rounded-full bg-slate-800/60 px-3 py-1 text-xs text-slate-400 hover:text-slate-200"
			on:click={load}
			disabled={loading}>{loading ? 'Refreshing…' : 'Refresh'}</button
		>
	</div>
	<Window {from} {to} onchange={setWindow} />
	<div class="mt-4">
		{#if stats}
			<Chart points={stats.points} name={key} rangeHours={span} height={320} large />
		{:else}
			<div class="grid h-80 place-items-center text-xs text-slate-500">No data yet</div>
		{/if}
	</div>
</section>

{#if stats && isCircular(key)}
	<!-- A bearing has a prevailing direction, not a high, low or records. -->
	<dl class="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
		<div class="rounded-xl border border-slate-800/80 bg-slate-900/50 p-3">
			<dt class="text-[11px] uppercase tracking-wide text-slate-500">Prevailing direction</dt>
			<dd class="mt-1 text-lg text-slate-100">{prevailingText(stats.prevailing, stats.prevailingShare)}</dd>
			<dd class="text-[11px] text-slate-500">most frequent direction, of the reports that give one</dd>
		</div>
	</dl>
{:else if stats}
	<dl class="mt-4 grid grid-cols-2 gap-3 sm:grid-cols-4">
		<div class="rounded-xl border border-slate-800/80 bg-slate-900/50 p-3">
			<dt class="text-[11px] uppercase tracking-wide text-slate-500">Window low</dt>
			<dd class="mt-1 text-lg text-slate-100">
				{fmt(stats.min, key)} <span class="text-xs text-slate-500">{unit(key)}</span>
			</dd>
			<dd class="text-[11px] text-slate-500">{localTime(stats.minAt)}</dd>
		</div>
		<div class="rounded-xl border border-slate-800/80 bg-slate-900/50 p-3">
			<dt class="text-[11px] uppercase tracking-wide text-slate-500">Window high</dt>
			<dd class="mt-1 text-lg text-slate-100">
				{fmt(stats.max, key)} <span class="text-xs text-slate-500">{unit(key)}</span>
			</dd>
			<dd class="text-[11px] text-slate-500">{localTime(stats.maxAt)}</dd>
		</div>
		<div class="rounded-xl border border-slate-800/80 bg-slate-900/50 p-3">
			<dt class="text-[11px] uppercase tracking-wide text-slate-500">Record high</dt>
			<dd class="mt-1 text-lg text-slate-100">
				{stats.hasRecord ? fmt(stats.recordHigh, key) : '—'}
				<span class="text-xs text-slate-500">{stats.hasRecord ? unit(key) : ''}</span>
			</dd>
			<dd class="text-[11px] text-slate-500">{stats.hasRecord ? stats.recordHighDate : '—'}</dd>
		</div>
		<div class="rounded-xl border border-slate-800/80 bg-slate-900/50 p-3">
			<dt class="text-[11px] uppercase tracking-wide text-slate-500">Record low</dt>
			<dd class="mt-1 text-lg text-slate-100">
				{stats.hasRecord ? fmt(stats.recordLow, key) : '—'}
				<span class="text-xs text-slate-500">{stats.hasRecord ? unit(key) : ''}</span>
			</dd>
			<dd class="text-[11px] text-slate-500">{stats.hasRecord ? stats.recordLowDate : '—'}</dd>
		</div>
	</dl>
{/if}
