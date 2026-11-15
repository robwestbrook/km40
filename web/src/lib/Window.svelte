<script>
	import { presetRange, ARCHIVE_START } from '$lib/units.js';

	export let from = '';
	export let to = '';
	export let onchange = () => {};
	export let active = '24h';

	const presets = [
		['24h', '24 hours'],
		['7d', '7 days'],
		['30d', '30 days'],
		['1y', '1 year'],
		['all', 'All']
	];

	function apply(preset, customFrom, customTo) {
		if (preset === 'custom') {
			if (!customFrom || !customTo) return;
			active = 'custom';
			from = customFrom;
			to = customTo;
			onchange(from, to, active);
			return;
		}
		const range = presetRange(preset, new Date());
		if (!range) return;
		active = preset;
		from = range.from;
		to = range.to;
		onchange(from, to, active);
	}

	// Read the inputs directly: a bound value is stale until the next update.
	let fromInput;
	let toInput;
	function custom() {
		if (fromInput.value && toInput.value) apply('custom', fromInput.value, toInput.value);
	}
</script>

<div class="flex flex-wrap items-center justify-center gap-2">
	{#each presets as [key, label]}
		<button
			type="button"
			class={`rounded-full px-3 py-1 text-xs font-medium transition ${
				active === key
					? 'bg-sky-500/20 text-sky-300 ring-1 ring-sky-400/40'
					: 'bg-slate-800/60 text-slate-400 hover:bg-slate-800 hover:text-slate-200'
			}`}
			on:click={() => apply(key)}
		>
			{label}
		</button>
	{/each}
	<span class="mx-1 h-4 w-px bg-slate-800"></span>
	<input
		bind:this={fromInput}
		type="date"
		min={ARCHIVE_START}
		aria-label="From date"
		class="rounded-md bg-slate-800/60 px-2 py-1 text-xs text-slate-300 outline-none focus:ring-1 focus:ring-sky-400/50"
		on:change={custom}
	/>
	<span class="text-xs text-slate-500">to</span>
	<input
		bind:this={toInput}
		type="date"
		aria-label="To date"
		class="rounded-md bg-slate-800/60 px-2 py-1 text-xs text-slate-300 outline-none focus:ring-1 focus:ring-sky-400/50"
		on:change={custom}
	/>
</div>
