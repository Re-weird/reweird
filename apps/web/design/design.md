# ReWeird web — design system

Read this before building or restyling any page in `apps/web`. It documents the
visual language already in use so new screens match instead of drifting.

## Philosophy

ReWeird should read as a **premium engineering instrument**, not an AI dashboard.

- Evidence first: measurement → derived facts → interpretation, always visually
  separated (see `.compare-grid`, `.rules-list`, `.interpretation-card` in
  `app/page.tsx`).
- Restrained color. Status color (pass/warn/fail) is the only place color
  carries meaning. Everything else is neutral gray/blue.
- No fake precision. Confidence reads as "68% confidence", not a decorated
  progress ring with neon glow. No invented metrics ("AI score", "efficiency").
- Quiet materials: flat surfaces, 1px borders, soft shadows. No glassmorphism,
  no gradients except the single dark hero photo panel, no neon/cyberpunk glow.

## Where the styling lives

| File | Role |
|---|---|
| `app/globals.css` | Base layout primitives (grid, nav, modal, cards) plus a **legacy** `:root` color-token block. That legacy block is intentionally superseded — see below. |
| `app/workbench.css` | **The current design system.** Redefines every color token for both themes, plus the "quiet instrument panel" component overrides (spacing, radius, font sizes) and all `.bench-*` / `.workbench-*` Workbench page styles. Imported in `app/layout.tsx` *after* `globals.css`, so its tokens win. |

**When you need a color or spacing value, read the tokens at the top of
`workbench.css`, not `globals.css`.** `globals.css` still defines a `:root`
block with the same variable names (an earlier, more saturated palette) — it's
dead weight kept only because deleting it isn't worth the diff risk. If you're
doing a larger cleanup pass, folding `globals.css`'s token block away entirely
(so there's one source of truth) is fair game.

Every screen shares these tokens via CSS custom properties, so a new page
automatically gets both themes for free as long as it only uses `var(--...)`
and never hardcodes a hex color.

## Color tokens (from `workbench.css`)

All colors are CSS custom properties on `:root` (dark, default) and
`:root[data-theme="light"]`. Toggling is a single `data-theme` attribute on
`<html>`, flipped by the theme button in `AppShell` (`app/page.tsx`).

| Token | Dark | Light | Use |
|---|---|---|---|
| `--bg` | `#101214` | `#f1f2f3` | Page background |
| `--sidebar-bg` | `#0c0e10` | `#fafbfc` | Sidebar background |
| `--surface` | `#171a1d` | `#ffffff` | Card / panel background |
| `--surface-2` | `#1b1f23` | `#f6f7f8` | Nested surface (chips, sub-rows) |
| `--surface-3` | `#24292e` | `#e9edf1` | Selected / active surface |
| `--field-bg` | `#121518` | `#fafbfc` | Form inputs |
| `--line` | `#30353a` | `#dcdfe3` | Default border |
| `--line-soft` | `#262b30` | `#e9ebee` | Divider inside a card |
| `--text` | `#edf0f2` | `#22282e` | Primary text |
| `--muted` | `#a0a8af` | `#616b76` | Secondary text |
| `--subtle-text` | `#8b949d` | `#69747f` | Tertiary / caption text |
| `--cyan` | `#94b3e7` | `#335f9b` | Accent (charts, links, focus) — muted blue, not neon |
| `--cyan-soft` | `#202c3c` | `#eaf0f9` | Accent tint background |
| `--action` | `#416cb0` | `#365f9d` | Primary button fill |
| `--green` | `#9abaad` | `#437361` | PASS / stable status |
| `--amber` | `#d7b988` | `#886327` | WARN status |
| `--red` | `#e3978f` | `#a94e48` | FAIL / intermittent status |
| `--violet` | `#aab2be` | `#616b76` | Rarely used, neutral accent |
| `--hover-bg` | `#20252a` | `#edf0f3` | Row/button hover |
| `--shadow` | `0 2px 4px #00000008` | `0 2px 4px #26364b04` | Card elevation — deliberately subtle |
| `--scope-bg` | `#121518` | `#f8fafb` | Signal-monitor plot background |

Status colors (`--green`/`--amber`/`--red`) are intentionally desaturated —
they read as "instrument panel indicator," not "traffic light." Don't reach
for a brighter red/green when something needs to feel urgent; increase
contrast in text/weight instead.

## Typography

- **Manrope** — body text, UI labels, headings. Loaded via Google Fonts in
  `globals.css`.
- **DM Mono** — anything numeric or machine-like: probe readings, timestamps,
  session IDs, the `eyebrow`/`bench-label` micro-labels, chart axes.
- Headings are tight: negative letter-spacing (`-0.2px` to `-1.3px` depending
  on size), weight 600–650, never heavier.
- Eyebrow/kicker labels: 9–10px, uppercase or as-typed, `--muted`/`--subtle-text`
  color, letter-spacing ~0.8–1px.

## Layout conventions

- Card radius: 9px for panels, 6–8px for buttons/chips/inputs. Nothing more
  rounded than that — no pill-shaped cards.
- Borders: 1px `var(--line)`, not shadows, do most of the separating.
- Shadows are barely visible (`--shadow`) — elevation comes from border +
  background contrast, not drop shadow.
- Sidebar is fixed width (`--sidebar`, 218px), grouped nav sections with a
  9px uppercase `DM Mono` group label (`.nav-label`).
- Content max-width ~1530px, generous padding (28–32px), consistent gap
  scale: 8 / 10 / 14 / 16 / 18 / 22px — pick from this scale, don't invent
  new gaps.
- One hero-style dark banner per page at most (see `.bench-welcome`) — it's
  the only place a background photo/gradient is allowed. Don't add a second
  one on a new page.

## Reusable component classes

Before inventing new CSS, check if one of these already does the job:

- `.panel` / `.bench-panel` — the standard card container.
- `.bench-panel-head` — card header row (eyebrow + title + right-aligned action).
- `.primary` / `.secondary` / `.text-button` — button variants.
- `.status-label.pass|warn|fail`, `.status-dot.stable|active|intermittent|idle`
  — evidence status indicators. Always use these three-state (or four with
  idle) vocabulary; don't invent new status colors per page.
- `.eyebrow` / `.kicker` / `.bench-label` — the small caps micro-labels used
  everywhere above a heading.
- `.spec-chips`, `.compare-grid`, `.rules-list` — evidence/expected-vs-observed
  layouts (see Diagnosis view).
- `.empty-state` / `.bench-empty` — empty-state block (icon + heading + one
  line of copy). Use this instead of a blank panel.
- `.modal` / `.modal-backdrop` — dialog pattern, already wired for focus trap
  and `Escape` (see `NewProjectModal` in `app/project-workflow.tsx`).

## Don'ts

Carried over from the original product brief — still binding:

- No neon/cyberpunk glow, no glassmorphism on every card, no huge rounded
  corners, no emoji in UI copy.
- No fabricated metrics or fake-precision numbers ("83.742% confidence" →
  "High confidence", optionally with the real rounded number beside it).
- No decorative charts on empty data — render "No measurement yet" instead of
  an empty axis.
- Never imply simulator data is physical hardware, or that PATCH executed
  something — those states have explicit locked/simulated badges already
  (`.live-badge`, `PATCH locked`, `SIMULATED`); reuse them.

## Adding a new page — checklist

1. Use only `var(--token)` colors from the table above — no hardcoded hex.
2. Build the page from the reusable classes in "Component classes" before
   writing new CSS. New CSS should extend `workbench.css`, not `globals.css`
   (which is legacy).
3. Give it an `.page-heading` (kicker/eyebrow + `<h1>` + one-line description)
   at the top, matching every other view in `app/page.tsx`.
4. Screenshot it in both light and dark mode, and at a ~390px mobile width,
   before calling it done — the sidebar collapses to an overlay under 800px
   (see the `@media` blocks at the bottom of `workbench.css`).
5. If it's a new top-level destination, add it to the `nav` array in
   `app/page.tsx` under the right group (Workspace / Diagnostic flow /
   Records) rather than creating a new nav rail.
