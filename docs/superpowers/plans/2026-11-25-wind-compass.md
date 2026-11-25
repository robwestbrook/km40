# Wind Direction Compass Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Show the current wind direction as a compass dial with a needle (wind speed in the dial's lower third) on the home card and the Wind direction detail page.

**Architecture:** A pure helper pair in `web/src/lib/units.js` (needle angle, speed text) is tested with `node --test`. A new SVG component `web/src/lib/Compass.svelte` draws the dial from those values. The home page swaps the dot chart for the dial on the `winddir` card only; the detail page keeps the dot chart and adds the dial beside it. Front-end only.

**Tech Stack:** SvelteKit 2 (Svelte 5, components written in legacy `export let` style like `Chart.svelte`), Tailwind 4, Node's built-in test runner.

**Spec:** `docs/superpowers/specs/2026-11-25-wind-compass-design.md`

## Global Constraints

- Front end only: no change to Go code, the API, the database or `go.mod`; no new npm dependencies.
- **No database work, ever** (user's standing rule). Tests use no database; visual checks use a stub API (Task 2), never `km40.db` or the real binary's data.
- Display units: wind speed in mph via `fmt(kt, 'wind')` / `unit('wind')`; the `winddir` element stays `°`.
- The needle points to where the wind comes **from** (same meaning as the existing "from WSW" wording).
- Calm (speed 0) and variable (speed > 0, no bearing): **no needle**; the speed still shows.
- Needle colour `#38bdf8` (existing sky accent); dial uses the slate palette already used by the cards.
- Must work at phone width (375 px) as the installed iPhone web app; no horizontal scroll.
- The Wind (speed) card and its detail page keep their current look, including the "from WSW (245°)" header line.
- Upper-left of the Wind direction card is unchanged ("245 °" and the compass point).
- Never push or deploy. Commit steps are local to branch `worktree-wind-mod`; ask the user at the start of execution whether commits are wanted, and skip the commit steps if not.
- Svelte 5 legacy-mode trap (see `HANDOFF.md`): do not derive values with component-local functions or `$:` that read other fresh assignments; pass expressions of props directly in the template or use functions imported from `units.js`.
- Work from `/mnt/ydrive/km40/.claude/worktrees/wind-mod`. The worktree already has what the web tests need; run `npm --prefix web ci` and `npm --prefix web run build` only if `web/node_modules` or `web/build` is missing.

## Review Focus

1. **North boundary:** a bearing of 360 (or 359.6 rounded) must point north, not off the dial. Pinned in Task 1.
2. **Calm with a bearing of 0:** METAR `00000KT` can arrive as direction 0, speed 0; no needle must show. Pinned in Task 1.
3. **Variable wind:** speed above 0 and no bearing; no needle, speed shown, no crash. Pinned in Task 1 and checked visually in Task 2.
4. **No data yet:** `summary.current` null or `windKt` null; the dial shows "—" and no needle. Pinned in Task 1.
5. **Narrow screens:** at 375 px the home card and the detail page (dial above the chart) do not overflow sideways. Checked in Tasks 2 and 3.

---

### Task 1: Needle angle and speed text helpers

**Files:**
- Modify: `web/src/lib/units.js` (add two exports after `bearingText`, near line 87)
- Test: `web/src/lib/units.test.js` (append; extend the import line at the top with the two names)

**Interfaces:**
- Produces: `needleAngle(deg: number|null|undefined, speedKt: number|null|undefined) -> number|null` and `speedText(speedKt: number|null|undefined) -> string`. Task 2 passes the results straight to `Compass`.

- [ ] **Step 1: Write the failing tests** (append to `web/src/lib/units.test.js`)

```js
test('needleAngle gives the bearing in 0-359, and null when no needle should show', () => {
	assert.equal(needleAngle(245, 9), 245);
	assert.equal(needleAngle(0, 5), 0);
	assert.equal(needleAngle(360, 5), 0); // north is never 360
	assert.equal(needleAngle(-10, 5), 350);
});

test('needleAngle is null for calm, variable and missing data', () => {
	assert.equal(needleAngle(0, 0), null); // calm reported as 00000KT
	assert.equal(needleAngle(245, 0), null);
	assert.equal(needleAngle(null, 6), null); // variable
	assert.equal(needleAngle(undefined, 6), null);
	assert.equal(needleAngle(Number.NaN, 6), null);
	assert.equal(needleAngle(null, null), null);
});

test('speedText reads as mph, and as a dash with no reading', () => {
	assert.equal(speedText(8), '9 mph'); // 8 kt = 9.2 mph
	assert.equal(speedText(0), '0 mph');
	assert.equal(speedText(null), '—');
	assert.equal(speedText(undefined), '—');
	assert.equal(speedText(Number.NaN), '—');
});
```

- [ ] **Step 2: Run to verify they fail**

Run: `npm --prefix web test`
Expected: FAIL, `needleAngle` / `speedText` are not exported from `units.js`.

- [ ] **Step 3: Implement in `web/src/lib/units.js`**
  - `export function needleAngle(deg, speedKt)`: return `null` when `deg` is null/undefined/NaN or `speedKt === 0`; otherwise `((deg % 360) + 360) % 360`.
  - `export function speedText(speedKt)`: return `'—'` for null/undefined/NaN, otherwise `` `${fmt(speedKt, 'wind')} ${unit('wind')}` ``.

- [ ] **Step 4: Run to verify they pass**

Run: `npm --prefix web test`
Expected: all tests pass (previously 15, now 18).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/units.js web/src/lib/units.test.js
git commit -m "Add needleAngle and speedText helpers for the wind compass"
```

---

### Task 2: Compass component and the home card

**Files:**
- Create: `web/src/lib/Compass.svelte`
- Modify: `web/src/routes/+page.svelte` (import line 6; the `<Chart ... />` block around line 143)

**Interfaces:**
- Consumes: `needleAngle`, `speedText` from Task 1.
- Produces: `Compass.svelte` with props `export let bearing = null` (degrees 0-359 or null, already from `needleAngle`), `export let speed = '—'` (display text), `export let size = 190` (pixels, width and height). Task 3 reuses it unchanged.

- [ ] **Step 1: Write `web/src/lib/Compass.svelte`**

Mirror the approved mockup `.superpowers/brainstorm/406474-1795628871/content/card-v3.html`: one `<svg viewBox="0 0 200 200">` sized by `size`, with `role="img"` and an `aria-label` that reads the bearing and speed (e.g. "Wind from 245 degrees, 9 mph", or "Wind direction unavailable" when `bearing` is null and speed is "—"). Contents: ring (r 82); four long ticks and four short diagonal ticks; N (sky colour), E, S, W labels; the needle as a `<g>` rotated with `transform="rotate({bearing} 100 100)"` rendered only `{#if bearing !== null}` (a long sky triangle toward the bearing and a short slate tail); the centre hub; and the `speed` text at y=152, 20px, semi-bold, centred. Needle and text must still read at `size` 120.

- [ ] **Step 2: Wire the home card in `web/src/routes/+page.svelte`**
  - Add `Compass` to the imports and `needleAngle, speedText` to the `units.js` import.
  - Replace the single `<Chart points={el.points} ... height={78} />` with `{#if el.element === 'winddir'}` a centred `<Compass bearing={needleAngle(summary.current?.windDir, summary.current?.windKt)} speed={speedText(summary.current?.windKt)} />` `{:else}` the existing `<Chart>` unchanged `{/if}`. Pass the expressions inline (legacy-mode trap).

- [ ] **Step 3: Build and run the unit tests**

Run: `npm --prefix web test && npm --prefix web run build`
Expected: tests pass; build succeeds with no new warnings about `Compass.svelte`.

- [ ] **Step 4: Visual check against a stub API (no database)**

Create the throwaway `/tmp/claude-1000/-mnt-ydrive-km40/911b495a-eabc-4551-b0e9-ac07b1738bd5/scratchpad/stub-api.mjs`, an `http` server on port 8080 (where `web/vite.config.js` proxies `/api`). It serves fixture JSON shaped like the real responses (read the `Series` struct in `internal/store/store.go` and the handlers in `internal/api/api.go`): `/api/summary` -> `{current, elements: [...]}` with a `winddir` element (a few `points`, `prevailing: 202.5`, `prevailingShare: 0.14`) and a `wind` element; `/api/latest` -> `{current}`; `/api/series?element=…` -> one series. The `current` object is chosen by the `STUB_CASE` env var: `wsw` (`windDir: 245, windKt: 8`), `calm` (`windDir: 0, windKt: 0`), `variable` (`windDir: null, windKt: 6`), `none` (`current: null`). Then run `STUB_CASE=wsw node stub-api.mjs` in the background, `npm --prefix web run dev`, and open the dev URL in headless Chromium (the `chrome-devtools` tools or `tools/cdp.mjs`).
Expected for each case:
- `wsw`: needle pointing WSW (lower left), "9 mph" in the lower third, upper-left of the card shows "245" and "WSW".
- `calm`: no needle, "0 mph", subline "calm".
- `variable`: no needle, "7 mph", subline "variable".
- `none`: no needle, "—", page does not error.
- At a 375 px wide viewport: the card does not overflow sideways.
Stop the stub and the dev server afterwards (`pgrep -x node` shows what to stop; never `pkill -f`).

- [ ] **Step 5: Commit**

```bash
git add web/src/lib/Compass.svelte web/src/routes/+page.svelte
git commit -m "Show wind direction as a compass dial on the home card"
```

---

### Task 3: Detail page

**Files:**
- Modify: `web/src/routes/element/[key]/+page.svelte` (import line 6; header block around line 79; chart block around line 107)

**Interfaces:**
- Consumes: `Compass` (Task 2) and `needleAngle`, `speedText` (Task 1).

- [ ] **Step 1: Header line only for the Wind speed page**

Change the condition `(key === 'wind' || key === 'winddir') && latest?.current?.windDir != null` to `key === 'wind' && latest?.current?.windDir != null`. The big "245 °" value stays on the Wind direction page.

- [ ] **Step 2: Dial beside the dot chart for `winddir`**

Inside the chart `<div class="mt-4">`, when `key === 'winddir'` wrap the content in `flex flex-col items-center gap-4 sm:flex-row sm:items-start`: a `<Compass bearing={needleAngle(latest?.current?.windDir, latest?.current?.windKt)} speed={speedText(latest?.current?.windKt)} />` (non-shrinking) and the existing chart block in a `min-w-0 w-full flex-1` container. Other keys render exactly as now. Add `Compass` and the two helpers to the imports.

- [ ] **Step 3: Run tests and build**

Run: `npm --prefix web test && npm --prefix web run build`
Expected: tests pass; build succeeds.

- [ ] **Step 4: Visual check with the Task 2 stub**

With `STUB_CASE=wsw`, open `/element/winddir` and `/element/wind`:
- `/element/winddir`: dial beside the dot chart at desktop width; dial stacked above the chart at 375 px with no sideways scroll; no "from WSW (245°)" line; big value "245 °".
- `/element/wind`: unchanged, "from WSW (245°)" line still present, no dial.
- `calm`, `variable`, `none` cases: no needle and no console errors on `/element/winddir`.
Stop the stub and dev server afterwards.

- [ ] **Step 5: Full verification**

Run: `go vet ./... && go test -count=1 ./... && npm --prefix web test`
Expected: Go packages ok, web tests 18 pass. (`go test` needs `web/build`; the build from Step 3 provides it.)

- [ ] **Step 6: Commit**

```bash
git add "web/src/routes/element/[key]/+page.svelte"
git commit -m "Add the compass dial to the wind direction detail page"
```

---

## After the tasks

Not part of this plan unless the user asks: merging into `main`, pushing, deploying with `deploy/deploy.sh`, and updating `HANDOFF.md` / `README.md`.
