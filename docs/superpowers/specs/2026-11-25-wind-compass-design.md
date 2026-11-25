# Wind direction compass — short design

Date: 2026-11-25. Size: small change. Status: awaiting the user's approval.

## Intent
The wind direction card shows a bearing as a number and a dot chart, which is hard to read at a glance.
The user wants a compass dial with a needle for the current direction. [said this session]

## What the user decided
- A compass rose with a **needle for the current direction**. [said this session]
- On the **home card** the dial **replaces the dot chart** (layout "A"). [said this session]
- **Upper left of the card stays as today**: the bearing ("245 °") and the compass point ("WSW"). The **wind speed goes
  in the lower third of the dial** ("9 mph"). The user first asked for degrees there, then asked to swap the speed and
  the direction; the final, confirmed version is the one above. [said this session]
- The **detail page keeps the dot chart**. [said this session]
- Approach: a hand-written SVG Svelte component (see Approaches). [user said "Yes" to my recommendation]

## Goals
- Direction readable at a glance on the home card and the detail page.
- One reusable component used at two sizes.

## Non-goals
- No API, database or data changes. No new dependencies. No gust, forecast or history on the dial.
- The Wind (speed) card and its detail page are not redesigned.
- No animation beyond what is needed (no rotating transitions) unless asked.

## Constraints
- Stack is fixed: SvelteKit 2, Tailwind 4, Chart.js. Display units stay °F / mph; the speed comes from the existing
  `wind` reading (`windKt`) via `convert`/`fmt` in `web/src/lib/units.js`.
- Must still work as the installed iPhone web app (safe-area layout unchanged) and at phone width.
- The user's standing rule: no database work, so tests use no database.

## Design
- New `web/src/lib/Compass.svelte`: props `bearing` (degrees or null), `speed` (display text), `size`.
  Draws the ring, N/E/S/W labels and tick marks, a needle rotated to the bearing (pointing to where the wind comes
  *from*, matching the existing "from SSW" wording), and the speed text in the lower third.
- Calm (speed 0) and variable (wind with no bearing): **no needle**; the speed still shows. [my call]
  Upper left already says "calm" / "variable" via the existing `subline` logic; no change needed there.
- Home card (`web/src/routes/+page.svelte`): for the `winddir` element only, render `Compass` in place of `Chart`.
  The Prevailing row stays.
- Detail page (`web/src/routes/element/[key]/+page.svelte`) for `winddir`: dot chart unchanged; `Compass` placed
  beside it; the "from WSW (245°)" line is removed on this page only, since the dial covers it. The Wind (speed)
  page keeps its line. [my call]
- Colours reuse the dashboard's slate/sky palette; the needle is the existing sky accent (#38bdf8).

## Approaches considered
1. **SVG Svelte component (chosen).** Small, no library, scales cleanly, same component for both pages. Cost: no
   Chart.js tooltips on the dial.
2. Chart.js polar chart with a custom needle plugin: matches other charts but needs plugin code and is built for many
   points, not one needle.
3. Plain HTML/CSS rotated div: lighter, but tick and label placement is awkward and scales worse on phones.

## Testing
- Pure helper in `units.js` (needle angle from a bearing; null for calm/variable) with `node --test` cases,
  including 0°, 360°→0, and null. No database.
- Visual check in headless Chromium on a built copy; the user confirms on their iPhone as before.

## Left to the builder
Exact dial sizes (card vs detail), tick styling, label font sizes, and the detail-page arrangement at narrow widths.
