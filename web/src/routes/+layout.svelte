<script>
	import '../app.css';
	import { ARCHIVE_SINCE } from '$lib/units.js';
</script>

<main class="mx-auto max-w-6xl px-4 py-8 sm:px-6">
	<slot />
	<footer class="mt-12 border-t border-slate-800 pt-4 text-center text-xs text-slate-500">
		METAR KM40 · Aberdeen / Monroe County Airport, Mississippi · data from
		Aviation Weather Center and IEM · records since {ARCHIVE_SINCE}
	</footer>
</main>
