# Handoff: Cairn marketing site

## Overview

A one-page marketing site for **Cairn** — a self-hosted issue tracker that treats
a coding agent as a first-class user. It ships as a single Go binary with the
frontend embedded, Apache-2.0, at
`https://github.com/alperkyoruk/cairn`.

The site's job is narrow and settled: **get the visitor to copy the install
command.** Everything else on the page is secondary. There is no signup, no
pricing, no waitlist, no newsletter, no analytics.

**Audience:** people already running coding agents daily who are annoyed their
tracker doesn't know about them, and self-hosters/homelabbers for whom the pitch
is single binary, SQLite, no cloud.

**Tone:** plain and technical. No exclamation marks, no growth-copy, no
"revolutionise". The product's own claims are concrete enough.

**Deliberately no product screenshots.** The design leads with real artifacts
instead — the install command, an MCP config JSON, and a rendering of the
product's core data concept. Do not add screenshots or illustrations to fill
space; if a section feels empty that is a layout problem, not a content gap.

## About the design file

`Cairn - site.dc.html` is a **design reference created in HTML** — a prototype
showing intended look, copy and behaviour. It is not production code to copy.

The task is to **recreate it in whatever environment fits**. There is no existing
site codebase, so pick appropriately: this is a single static page with two tiny
pieces of interactivity, so plain HTML + CSS + ~30 lines of JS is the honest
choice. Astro or a single-file Vue/Svelte page are also fine. Do not reach for a
framework with a build pipeline heavier than the page.

The prototype's markup is a streaming component format and uses inline styles
throughout — **do not lift it**. Read it for layout, measurement, copy and
behaviour, then write clean HTML with a real stylesheet and CSS custom
properties.

## Fidelity

**High-fidelity.** Colors, type sizes, spacing and states are final. Match them.
Every value comes from the token sheet below; do not introduce values outside it.

## Constraints

- **Must work with JavaScript disabled** except the two interactive bits (see
  Interactions). The install command must be readable and selectable without JS —
  render the default tab server-side / statically, and let JS only swap it.
- **No webfonts fetched at runtime.** The type stack lists Inter first so a
  self-hosted subset is used when present, and falls through to the system stack
  when not. If you self-host Inter, commit a woff2 subset with
  `font-display: swap`, weights 400 and 500 only — the design never goes heavier
  than 500.
- **No icon library.** Two inline SVGs total (see Assets).
- **No analytics, no tracking, no cookie banner.** The footer says so; keep it
  true.
- **No CSS framework.** Plain CSS with custom properties.
- Dark only. The product app has a light theme pending; the site does not need
  one. Set `color-scheme: dark`.
- Desktop-first, must not break narrow. See Responsive.

## Design tokens

Copy verbatim as the site's token layer.

```css
:root {
  color-scheme: dark;

  /* ground */
  --bg:          #161826;
  --surface:     color-mix(in srgb, #232532 55%, #161826);
  --panel:       #232532;

  /* ink */
  --text:        #e9e9ed;
  --text-muted:  #cfd3e5;
  --text-dim:    #9397ab;
  --text-faint:  #75798c;
  --text-ghost:  #595d6c;

  /* rules and hairlines */
  --rule:        color-mix(in srgb, #e9e9ed 16%, transparent);
  --hairline:    #3f424d;

  /* roles */
  --accent:      #9184d9;
  --accent-lit:  #b5abfc;   /* accent text on dark; links on hover */
  --accent-mid:  #968ae0;   /* filled dots */
  --blocked:     oklch(0.72 0.105 70);
  --blocked-tint: oklch(0.72 0.105 70 / 0.13);

  /* type */
  --font: "Inter", system-ui, -apple-system, "Segoe UI", sans-serif;
  --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;

  /* space (0.7x density scale, inherited from the product) */
  --s-1: 3px;  --s-2: 6px;  --s-3: 8px;  --s-4: 11px;
  --s-6: 17px; --s-8: 22px;

  /* radii + elevation: an edge, then ambient darkness */
  --r-sm: 4px; --r-md: 8px; --r-lg: 14px;
  --e-1: 0 0 0 1px #3f424d;
}
```

### The three rules that make it look right

1. **Rules fade at their ends.** Every horizontal divider — and there is one
   above every section — is a gradient that goes transparent 48px from each end,
   never a clean `border-top`:

   ```css
   background: linear-gradient(to right,
     transparent, var(--rule) 48px,
     var(--rule) calc(100% - 48px), transparent) no-repeat top / 100% 1px;
   padding-top: 64px;
   ```

   The worklog rail uses the vertical equivalent, fading 20px from each end, at
   x=6px: `no-repeat 6px 0 / 1px 100%`.

2. **The accent is a line and a glow, never a flood.** Buttons are outlined —
   1px accent border on transparent — never solid-filled. No section has a
   colored background. No gradient hero.

3. **Never pure black or pure white**, and no large saturated fill anywhere.

Layout is **left-aligned and asymmetric** throughout. Headings are flush left,
content hugs the left edge, whitespace lives on the right. No centred text
blocks, no centred hero.

## Page structure

Single column, `max-width: 1100px`, `margin: 0 auto`, `padding-inline: 40px`.
Every section below the hero opens with the fading rule + `padding-top: 64px`
and closes with `padding-bottom: 96px`.

Section order: nav · hero · premise · concept · MCP · quickstart · what it
doesn't do · license & contributing · footer.

### Nav

`display: flex; align-items: center; gap: var(--s-4); padding: var(--s-6) 40px`.
No bottom border. Not sticky.

Cairn mark (17px, `fill: var(--accent)`) + `cairn` wordmark at 18px weight 500,
with `margin-right: auto`. Then anchor links at 14px inheriting text color,
going `var(--accent)` on hover: `Concept` `MCP` `Install`, then `GitHub` at
`--text-dim`.

### Hero

`padding: 80px 40px 96px`.

1. **Kicker** — a 22px × 1px accent bar, then `SELF-HOSTED ISSUE TRACKER` in
   mono 11.5px, `letter-spacing: 0.08em`, uppercase, `var(--accent)`.
   `margin-bottom: var(--s-8)`.
2. **Headline** — 56px, `line-height: 1.06`, weight 500, `max-width: 19ch`:
   *An issue tracker your coding agent can actually use.*
3. **Tagline** — 18px, `line-height: 1.6`, `--text-muted`, `max-width: 62ch`,
   `text-wrap: pretty`, `margin-bottom: 44px`. This copy is the author's own and
   must not be rewritten:

   > A cairn is a stack of stones left on a trail so whoever comes next knows
   > the way. Every task here carries a note for whoever picks it up, human or
   > agent.

4. **The install block** — the page's whole purpose. See below.
5. **Follow-up line** — 13px `--text-dim`: "Then open `http://127.0.0.1:7777`.
   That is the whole install." beside a 12.5px accent link, *All builds &
   checksums*, to `/releases/tag/v0.1.0`.
6. **Facts row** — 12.5px `--text-faint`, separated by `·` in `--hairline`:
   One file, ~5MB · No runtime, no dependencies · SQLite · Frontend embedded ·
   Works airgapped · Apache-2.0.

#### The install block

A three-option segmented control above a command strip.

**Segmented control.** `display: inline-flex; overflow: hidden;
border: 1px solid var(--rule); border-radius: var(--r-md)`. Options are labels
wrapping a visually-hidden radio, `padding: 7px 12px`, 13px, with
`border-left: 1px solid var(--rule)` between them. The checked option takes
`color: var(--accent)` and `box-shadow: inset 0 0 0 1px var(--accent)`; unchecked
hover takes `background: color-mix(in srgb, var(--text) 7%, transparent)`.

Options and their commands — **exact strings, do not reword**:

| tab | command |
|---|---|
| **Binary** *(default)* | `tar -xzf cairn_v0.1.0_darwin_arm64.tar.gz && ./cairn` |
| **Docker** | `docker run -d -p 127.0.0.1:7777:7777 -v cairn-data:/data ghcr.io/alperkyoruk/cairn:latest` |
| **From source** | `make build && ./cairn` |

**Command strip.** A flex row, `--r-md`,
`background: var(--surface)`, `box-shadow: inset 0 0 0 1px var(--hairline)`. It
spans the **full content column** — do not cap its width; the Docker command is
~91 characters and clips at anything under about 800px of track.

- Left: a scrolling region, `flex: 1`, `overflow-x: auto`,
  `padding: var(--s-4) var(--s-6)`, holding a non-selectable `$` prompt in
  `var(--accent-lit)` and then the command in mono **12.5px**,
  `white-space: nowrap`, `--text-muted`.
- Right: a `Copy` button, `flex: none`, accent-outlined, square left corners
  (`border-radius: 0 var(--r-md) var(--r-md) 0`), with only a 1px left border in
  `--hairline` so it reads as part of the strip.

**The `$` must not be copied.** Put it in its own element and either exclude it
from the copy payload (the JS copies a data attribute, not the DOM text) or set
`user-select: none` on it.

### Premise

`h2` 34px, `max-width: 24ch`: *Agent support is not a plugin.*

Two equal columns, `gap: 48px`, `max-width: 940px`. Each has an 11px uppercase
`letter-spacing: 0.08em` heading over a 15px `line-height: 1.65` paragraph.

- **How every other tracker works** — heading `--text-dim`, body `--text-dim`:
  "Designed human-first, then agent access is bolted on afterwards as a
  third-party add-on wrapping the public API. The agent is a robot pretending to
  be a user: it posts comments, it moves cards, and the schema has no idea it
  exists. Nothing in the data model distinguishes a decision a person made from a
  step a process took."
- **How Cairn works** — heading `var(--accent)`, body `--text-muted`: "Agent
  behaviour is in the schema and in the rules. Agents are named actors with their
  own tokens. Two transitions are reserved for the human and cannot be made by an
  agent at all — deciding what gets worked on, and deciding something is
  finished. Everything else an agent does, it does as a first-class user of the
  same server."

Framed as a premise, not a comparison table. **Do not turn this into a feature
matrix and do not name competitors.**

### Concept — the most important section

`h2` 34px, `max-width: 26ch`: *Two records per task, and they are not the same
kind of thing.* Then a 16px `--text-dim` lede, `max-width: 64ch`: "This is the
whole idea. One is the note on top of the cairn — small, current, overwritten
every time. The other is the trail behind it — append-only, never edited, never
deleted."

Two equal columns, `gap: 48px`. **The two sides must have opposite geometry —
this is the section's entire argument and the thing most likely to be lost in the
port:**

| | left: `state` | right: `worklog` |
|---|---|---|
| enclosure | a panel — `--panel` fill, `--r-lg`, `--e-1`, plus a 2px inset left mark in `var(--blocked)` | none — flush on the page ground |
| structure | three named fields, one byline, no chronology | a stack of dated prose down a 1px vertical rail |
| field keys | mono 11px, `letter-spacing: 0.06em`, `--text-dim` | none; prose only |
| reads as | a record with slots | a history |

A panel reads as a record; a rail reads as a history. **Do not give the worklog a
card, and do not give state a timeline.**

Each column has a header row — `h3` 21px lowercase (`state` / `worklog`) beside
a mono 11.5px `--text-faint` legend (`one row · overwritten in place` /
`append-only · 28 entries`) — and closes with a 14px `--text-dim` paragraph.

**The state panel.** Header row: `State` (11px uppercase, `--text-dim`) left, a
blocked status mark right — 8×8 square, `background: var(--blocked)`,
`transform: rotate(45deg)`, plus the word `blocked` at 12px in the same color,
`gap: 7px`.

Fields stacked with `gap: var(--s-6)`:

1. `blocked_on` **first and promoted** — its own box, `padding: var(--s-4)
   var(--s-6)`, `--r-md`, `background: var(--blocked-tint)`, key in
   `var(--blocked)`, value at **15px `var(--text)`** — larger than the other
   fields, because on a blocked task it is the most important text on the screen.
2. `where_i_left_off` — key `--text-dim`, value 14px `#e4e7f5`.
3. `next_step` — same.
4. Byline, 12px `--text-faint`: `written by codex · 1d ago`.

Closing line: "Three fields, one byline, no history. An agent picking the task up
reads this and nothing else. So does the human, at a glance, from the board."

**The worklog rail.** Wrapper `padding-left: var(--s-3)` painting the vertical
fading 1px gradient at x=6px. Four entries, each
`display: grid; grid-template-columns: 11px 1fr; gap: var(--s-6);
padding-bottom: var(--s-8)`.

- Entry 1 is the collapsed group: an 11×11 hollow ring
  (`background: var(--bg); box-shadow: inset 0 0 0 1px var(--hairline)`) beside
  mono 12px `--text-faint`: `23 earlier entries · 4 – 19 Aug`.
- Entries 2–4 have a 5×5 marker dot at `margin: 6px 0 0 3px` —
  `var(--accent-mid)` when the entry carries a status change, `--text-ghost`
  otherwise — except a transition **into** blocked, which uses a 7×7 rotated
  square in `var(--blocked)`.
- Entry content: a meta line (actor 12.5px `--text-muted`; an optional status
  chip; timestamp pushed right with `margin-left: auto`, mono 11px
  `--text-ghost`, format `20 Aug 09:12`), then `what_was_tried` at 13.5px
  `--text-muted`, then `outcome` at the same size in `--text-dim`. **The
  luminance drop is the only thing separating the two — no labels, no icons.**
- The status chip: mono 11px, `padding: 2px 8px`, `--r-sm`,
  `background: color-mix(in srgb, var(--text) 6%, transparent)`, reading
  `queue → active` with the arrow at `--text-faint` and the destination status in
  its own color. A transition into blocked uses `background: var(--blocked-tint)`.

Copy for the three visible entries is in the prototype; use it verbatim — it is
real text from a real task and the awkward length is the point.

Closing line: "What was tried, what happened, and the status change it came with.
Nothing here is ever edited or deleted, so the record of how a task got where it
is survives the agent that wrote it."

### MCP

`h2` 34px, `max-width: 24ch`: *Your agent talks to it over MCP.* Then a 16px
`--text-dim` lede, `max-width: 64ch`: "Register the agent in the web UI, copy the
token once, paste it into the agent's MCP config. Streamable HTTP at `/mcp`, same
server, same database. The agent never sees the web interface — that is yours."

Grid `minmax(0, 1.15fr) minmax(0, 1fr)`, `gap: 48px`, `align-items: start`.

**Left: the config.** `<pre>` at `padding: var(--s-8)`, `--r-lg`,
`background: var(--surface)`, `box-shadow: inset 0 0 0 1px var(--hairline)`, mono
13px, `line-height: 1.75`, `--text-muted`, `overflow-x: auto`. JSON keys in
`var(--accent-lit)`, string values in `#e4e7f5`, punctuation in `--text-muted`.

```json
{
  "mcpServers": {
    "cairn": {
      "type": "http",
      "url": "http://localhost:7777/mcp",
      "headers": {
        "Authorization": "Bearer crn_7fJq…3uQg"
      }
    }
  }
}
```

⚠️ **Verify this against the real server before shipping.** The shape is a
reasonable guess at the MCP client config format, not something confirmed from
the repo. Same for the capability list below — it is written descriptively
precisely because the actual tool names were not available. If the repo exposes
concrete tool names, use them.

**Right: capabilities.** `What the agent can do` as an 11px uppercase
`--text-dim` heading, then five rows on `grid-template-columns: 11px 1fr`, each
with a 5×5 `var(--accent-mid)` dot at `margin: 8px 0 0 3px` and 14px
`--text-muted` text:

- Read the board and read one task with its state and full worklog
- Write the task's state — where it left off, the next step, what it is blocked on
- Append a worklog entry: what it tried, what happened
- Move a task through the statuses it is allowed to move it through
- File a new task in the backlog

Then, `margin-top: var(--s-8)`, a boxed counterpoint — `padding: var(--s-6)`,
`--r-md`, `background: var(--surface)` — headed `What it cannot do` in
`var(--blocked)`, body 13.5px `--text-dim`: "Queue a task from the backlog, or
mark anything done. Those two transitions are the human's, enforced on the server
rather than by prompt. You decide what gets worked on and you decide when it is
finished."

That box is the sharpest thing on the page for the target audience. Do not soften
it and do not move it below the fold of its own section.

### Quickstart

`h2` 34px: *Running in three steps.* Then three steps in a column,
`gap: 48px`, `max-width: 820px`. Each is
`display: grid; grid-template-columns: 34px 1fr; gap: var(--s-8);
align-items: start` with a mono 13px `var(--accent)` number (`01` `02` `03`) and
a content block: `h3` 19px, a 14.5px `--text-dim` paragraph at `max-width: 60ch`,
then whatever the step needs.

**01 — Unpack it and run it.** "One file with the frontend inside it. Nothing to
install alongside, nothing to point at a database, no runtime to provision."
Command strip (same styling as the hero's, no copy button):
`tar -xzf cairn_v0.1.0_darwin_arm64.tar.gz && ./cairn`. Then three 12.5px
`--text-faint` lines:

- macOS and Linux on amd64 and arm64, Windows on amd64, with `checksums.txt`.
- Docker instead: the image is 17.3MB, `linux/amd64` and `linux/arm64`, built
  `FROM scratch` and running as an unprivileged uid.
- From source: Go 1.25+ and Node 24+, then `make build && ./cairn`.

**02 — Name yourself.** "Open the root URL and pick a username and a password. No
email, no confirmation step, no account. This is your server — the name you pick
is the one that appears next to every task you touch." Then a 12.5px
`--text-faint` note: "There is no reset email and never will be. If you forget
it, run `cairn --reset-password` on the machine it runs on."

**03 — Register an agent.** "Give it a name — `claude`, `codex` — and Cairn
issues a token. It is shown exactly once and stored only as a hash. Paste it into
that agent's MCP config and it starts filing its own worklog." Then two buttons:
`Read the README` (primary, accent outline) → the repo, and `Open an issue`
(secondary, `--rule` outline) → `/issues`.

### What it does not do

`h2` 34px, `max-width: 26ch`: *What it does not do, and will not.* Then 16px
`--text-dim`, `max-width: 64ch`: "This is a closed list, not a roadmap. Every one
of them is a deliberate omission, and the tool is smaller and faster because of
it."

A wrapping flex row, `gap: var(--s-2) var(--s-4)`, `max-width: 920px`, of 14.5px
`--text-faint` terms separated by `·` in `--hairline`. Reads as a wall of
absence, which is the point:

Gantt · calendar view · saved views · due dates · reminders · recurring tasks ·
labels · tags · priorities · estimates · sprints · cycles · milestones ·
attachments · comments · subtasks · task hierarchy · dependencies · CalDAV ·
SMTP · notifications · toasts · mobile app · desktop app · teams · roles ·
invitations · sharing · assignee pickers · mentions · activity feeds

Then, `margin-top: 44px`, the closing statement at 19px `line-height: 1.5`
`#e4e7f5`, `max-width: 52ch`:

> There is one user. There is no one to notify, no one to assign to, and no one
> to share with.

**This list is closed.** Do not add a roadmap section, and do not turn any of
these into "coming soon".

### License & contributing

Two equal columns, `gap: 48px`, `max-width: 940px`. `h3` 21px over 14.5px
`--text-dim` body.

- **Apache-2.0** — "Use it, fork it, run it inside a company. The whole point is
  that it is yours once it is on your machine — there is no hosted tier to upsell
  you to and no telemetry to switch off."
- **Contributing** — "Go on the server, Vue 3 on the frontend, no CSS framework
  and no icon library — every dependency ends up in the binary and in the supply
  chain of a security-adjacent self-hosted tool. Read the list above before
  proposing a feature." Then `Repository` and `License` secondary buttons.

### Footer

Fading rule, `padding-top: 44px`, then a wrapping flex row, `gap: var(--s-8)`,
`align-items: baseline`: the cairn mark (15px, `fill: var(--text-dim)`) + `cairn`
at 16px `--text-dim`; then 13px `--text-faint` links — GitHub, README, Issues,
Apache-2.0; then, pushed right with `margin-left: auto`, mono 11.5px
`--text-ghost`: `no analytics on this page`.

## Buttons

Two variants, both outlined, never filled.

```css
.btn {
  display: inline-flex; align-items: center; justify-content: center; gap: 6px;
  font-family: var(--font); font-weight: 500; font-size: 14px; line-height: 1.2;
  padding: var(--s-2) calc(var(--s-3) * 1.2);
  border: 1px solid transparent; border-radius: var(--r-md);
  background: transparent; color: var(--text);
  cursor: pointer; text-decoration: none;
}
.btn-primary { color: var(--accent); border-color: var(--accent); }
.btn-primary:hover  { background: color-mix(in srgb, var(--accent) 12%, transparent); }
.btn-primary:active { background: color-mix(in srgb, var(--accent) 22%, transparent); }
.btn-secondary { border-color: var(--rule); }
.btn-secondary:hover  { background: color-mix(in srgb, var(--text) 7%, transparent); }
.btn-secondary:active { background: color-mix(in srgb, var(--text) 14%, transparent); }
```

Links: `color: var(--accent)`, no underline, `var(--accent-lit)` on hover.

Focus everywhere: `:focus-visible { outline: 2px solid var(--accent);
outline-offset: 2px; }`. Never the browser default.
Selection: `::selection { background: color-mix(in srgb, var(--accent) 30%, transparent); }`.

## Interactions & behavior

There are exactly two, and both must degrade gracefully.

1. **Install tab switch.** Three radio inputs swap the command string. Implement
   as real radios in a `fieldset` with a visually-hidden legend, so it works
   with keyboard and screen readers and — if the three commands are all rendered
   and CSS-toggled via `:has()` / sibling selectors — without JS at all.
   Default tab is **Binary**. No animation.
2. **Copy button.** `navigator.clipboard.writeText()` on the current command,
   then the label becomes `Copied` for ~1600ms and reverts. Wrap in a
   `try`/`catch` — a failed write should leave the label alone rather than lie.
   Exclude the `$` prompt from the payload. No toast.

Nav links are same-page anchors to `#concept`, `#mcp`, `#install`. Add
`scroll-behavior: smooth` on `:root` guarded by
`@media (prefers-reduced-motion: no-preference)`.

**No other animation.** No scroll reveals, no parallax, no counters, no
fade-ins. The page is static and that is deliberate.

## Responsive

Desktop-first; must not break narrow, does not need to be beautiful there.

- Below ~900px: collapse the premise, concept, MCP and license grids to one
  column. The concept's two columns stack state-then-worklog.
- The hero headline should drop to about 40px below 900px and 32px below 620px.
- The command strip already scrolls horizontally; keep that as the narrow
  fallback rather than wrapping the command, so it stays copy-pasteable.
- Nav links may wrap. Do not build a hamburger for four anchors.

## Accessibility

- One `h1`, sections in order, no heading levels skipped.
- The `$` prompt and the decorative dots/diamonds get `aria-hidden="true"`.
- The copy button announces its state change — use `aria-live="polite"` on the
  label, or swap `aria-label`.
- The `<pre>` config block is not interactive; do not trap focus in it.
- Accent-on-ground is tuned to about 3:1 — fine for the kicker, chrome and large
  text, **not** for body copy. Paragraph-size accent text uses
  `var(--accent-lit)`, never `var(--accent)`.

## Assets

Two inline SVGs, no library, no image files.

**The cairn mark** — three stacked stones, on a 16×16 viewBox, `fill` set by
context:

```html
<svg viewBox="0 0 16 16" aria-hidden="true">
  <path d="M8 1.4 10.4 4H5.6L8 1.4Z"></path>
  <path d="M4.6 5.2h6.8l1.5 2.6H3.1l1.5-2.6Z"></path>
  <path d="M2.2 9h11.6l1.6 3.4H.6L2.2 9Z"></path>
</svg>
```

Used at 17px in the nav, 15px in the footer. This is also the favicon — export it
as an SVG favicon with `fill="#9184d9"` on `--bg`.

No other icons. The status diamond, the marker dots and the hollow ring are all
CSS shapes, not SVG.

## Open items for whoever builds this

1. **Verify the MCP config shape and the capability list** against the real
   server. Flagged above; it is the only place on the page where the copy is a
   guess.
2. **The tarball filename in the hero is macOS-arm64-specific.** Two honest
   options: detect the visitor's platform from the UA and swap the filename, or
   genericise to `cairn_v0.1.0_<os>_<arch>.tar.gz`. Currently the former is
   unimplemented and the latter is not written. Pick one; do not leave a
   Darwin-only command as the universal install line.
3. **The version is hard-coded** (`v0.1.0`, ~5MB, 17.3MB image). Either template
   these from the GitHub releases API at build time, or accept that they need a
   manual bump per release and note it in the repo.
4. **A `<meta>` block is not designed.** Write it: title, description drawn from
   the tagline, an OG image (a dark card with the mark and the headline is
   sufficient), and `theme-color: #161826`.

## Files in this bundle

- `Cairn - site.dc.html` — the design reference. Open it in a browser and read
  it alongside this README.
- `_ds/nocturne-.../styles.css` — the Nocturne design system stylesheet the
  prototype composes (`.nav`, `.btn`, `.seg`, `.field`, `.input`). Reference for
  the component layer only; **do not vendor it** — the site needs about forty
  lines of CSS beyond the tokens.
- `support.js` — the prototype's runtime. Not part of the deliverable.

## The premise, so the copy doesn't drift

Existing trackers are designed human-first and bolt agent access on afterwards as
a third-party add-on. Cairn does the opposite: agent behaviour is baked into the
schema and the rules. The site's job is to make that legible in about fifteen
seconds, and then get out of the way of the install command.

The name is the product thesis. A cairn is a stack of stones left on a trail so
whoever comes next knows the way. Every task carries a note for whoever picks it
up, human or agent.
