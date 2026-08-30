# Handoff: Cairn web interface

## Overview

Cairn is a self-hosted issue tracker that treats a coding agent as a first-class
user. It ships as a single Go binary with the frontend embedded via `embed.FS`.
There is exactly **one human user** — their own machine or their own server. The
other users are coding agents, and they never see this interface; they talk to
the same server over MCP.

The interface's job is therefore narrow: let one person see at a glance what
their agents have been doing, and what needs a decision from them. The person
opens the root URL many times a day for five seconds at a time. Density and
scannability beat spaciousness everywhere.

This bundle covers: design tokens, the six-status vocabulary, task detail, the
board, project detail, project creation, empty states, the relative-time
convention, agents & tokens, login, and first-launch setup.

## About the design files

`Cairn - interface spec.dc.html` is a **design reference created in HTML** — a
single scrollable spec page showing every screen at desktop width with design
notes alongside. It is not production code to copy.

The task is to **recreate these designs in the target codebase**: Vue 3 + Vite,
Composition API, `<script setup>`. The spec page's own markup is a streaming
prototype format and should not be lifted; read it for layout, measurement and
copy, then build idiomatic Vue single-file components.

The spec page renders the whole app on one canvas, one section per screen. Each
section is anchored (`#tokens`, `#status`, `#task`, `#board`, `#project`,
`#newproject`, `#empty`, `#time`, `#agents`, `#entry`, `#open`).

## Fidelity

**High-fidelity.** Colors, type sizes, spacing, and states are final. Recreate
the UI to match. Every value comes from the token sheet in
`tokens.css` below — do not introduce values outside it.

Two things are explicitly **not** done and should not be invented:
- **Light theme.** Deferred. Scaffold `prefers-color-scheme` + an explicit
  toggle (the moon icon in the nav is already placed), but leave the light
  token block empty pending design. Only the ground and ink token groups will
  need overriding; roles, type, space and radii are hue-stable.
- **Anything on the closed "do not design" list** — see the end of this README.

## Hard technical constraints

These are not negotiable.

- **Vue 3 + Vite**, Composition API, `<script setup>`.
- **Must work completely offline.** The whole app is embedded in a single binary
  people run on their own machines, sometimes airgapped. No Google Fonts, no
  CDN, no runtime fetch of anything. Any font or asset must be vendorable.
- **No CSS framework, no icon library.** Plain CSS with custom properties.
  Every dependency ends up in the binary and in the supply chain of a
  security-adjacent self-hosted tool.
- **Desktop-first.** Must not break on a narrow window; phone use is not a goal.
- **No animation** beyond what makes a state change legible (one 120ms
  transition, scoped to status marks and the actions row).
- Light and dark following `prefers-color-scheme` with an explicit toggle
  (dark authored now, light later).

### Fonts

The type stack is:

```css
--font: "Inter", system-ui, -apple-system, "Segoe UI", sans-serif;
--mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
```

Inter is listed first so a vendored subset is used when present, and the system
stack carries it when not. If you vendor Inter, vendor a woff2 subset with
`font-display: swap` and weights 400 and 500 only — the design never goes
heavier than 500.

### Icons

Eleven inline SVGs total, no library: the cairn mark (three stacked stones), the
theme moon, the agent chip glyph, the status check, a chevron, and a warning
triangle. All drawn to Phosphor proportions at 1.5px stroke on a 16px box, so a
later swap to the real Phosphor set is a find-and-replace rather than a
redesign. Extract them verbatim from the spec page into a small
`components/icons/` directory of single-file components.

## Design tokens

Copy this verbatim as `web/src/styles/tokens.css` and import it once from the
app entry. Nothing in the app should hard-code a color, a size, or a spacing
value that is not one of these.

```css
/* cairn tokens. dark is the authored theme; light overrides
   the ground and ink groups only — the rest is hue-stable. */
:root {
  color-scheme: dark;

  /* ground */
  --bg:             #161826;
  --surface:        color-mix(in srgb, #232532 45%, #161826);
  --surface-raised: #232532;

  /* ink */
  --text:           #e9e9ed;
  --text-muted:     #b2b6ca;
  --text-dim:       #75798c;
  --text-faint:     #595d6c;

  /* rules — fade to transparent 48px from each end */
  --rule:           color-mix(in srgb, #e9e9ed 8%, transparent);
  --rule-strong:    color-mix(in srgb, #e9e9ed 16%, transparent);

  /* roles */
  --accent:         #9184d9;
  --accent-tint:    color-mix(in srgb, #9184d9 14%, transparent);
  --accent-deep:    #423a6a;
  --blocked:        oklch(0.72 0.105 70);
  --blocked-tint:   oklch(0.72 0.105 70 / 0.13);

  /* type */
  --font: "Inter", system-ui, -apple-system, "Segoe UI", sans-serif;
  --mono: ui-monospace, SFMono-Regular, "SF Mono", Menlo, Consolas, monospace;
  --t-xs: 11px;  --t-sm: 12px;  --t-base: 13px;
  --t-md: 15px;  --t-lg: 19px;  --t-xl: 25px;

  /* space (0.7x density) */
  --s-1: 3px;  --s-2: 6px;  --s-3: 8px;  --s-4: 11px;
  --s-6: 17px; --s-8: 22px; --s-12: 34px;

  /* radii + elevation: an edge, then ambient darkness */
  --r-sm: 4px; --r-md: 8px; --r-lg: 14px;
  --e-1: 0 0 0 1px #3f424d;
  --e-2: 0 0 0 1px #595d6c, 0 6px 18px rgb(0 0 0 / .55);

  /* one transition in the whole app, for status changes only */
  --motion: 120ms ease-out;
}

@media (prefers-reduced-motion: reduce) { :root { --motion: 0s; } }
```

### Notes on the tokens

- **Four ink steps, not two.** This interface uses dimness rather than size for
  hierarchy: a done task and a stale timestamp both recede by losing luminance,
  never by shrinking. `--text` for titles, state prose and the values a reader
  is looking for; `--text-muted` for next-step and worklog outcomes;
  `--text-dim` for actor names, times and field labels; `--text-faint` for
  empties and done rows.
- **Rules fade at their ends.** Every horizontal rule and every table row rule
  is a gradient that goes transparent 48px from each end, never a clean stop.
  Implement as a background strip on the row/container, not a border, so the
  fade spans the whole width rather than each cell:

  ```css
  background: linear-gradient(to right,
    transparent, var(--rule) 48px,
    var(--rule) calc(100% - 48px), transparent) no-repeat bottom / 100% 1px;
  ```
- **The accent is a line and a glow, never a flood.** Buttons are outlined
  (1px accent border on transparent), never solid-filled. No large surface ever
  takes the accent as a background.
- **`--blocked` is the one hue outside the accent** — an amber at the accent's
  own lightness and lower chroma, so it never outshouts it. Used as a mark, a
  hairline, and a 13%-alpha tint. Never as a fill.
- **Never pure black or pure white.** Shadows are the exception: ambient
  darkness mixed from black is a shadow, not a color.
- Spacing is a 0.7× density scale on purpose. Use the variables.
- Focus is always `outline: 2px solid var(--accent); outline-offset: 2px` on
  `:focus-visible`. Never the browser default.
- Selection is `background: color-mix(in srgb, var(--accent) 30%, transparent)`.
- Disabled controls drop to `opacity: 0.45`.

## The one concept the interface has to make legible

Every task carries two separate records, and a reader must never confuse them.

**state** — a single summary, overwritten in place, always current. Three
fields: `where_i_left_off`, `next_step`, `blocked_on`. Plus who wrote it and
when. Exactly one per task. It is the note on top of the cairn.

**worklog** — append-only. Never edited, never deleted. Each entry:
`what_was_tried`, `outcome`, who, when, and the status change it accompanied if
there was one. This is the trail behind the cairn.

**The design solves this with opposite geometry, and that is the single most
important thing to preserve in the port:**

| | state | worklog |
|---|---|---|
| enclosure | a panel: raised surface, 14px radius, its own edge | none — flush on the page ground |
| structure | three named fields, one byline, no chronology | a stack of dated prose down a vertical rail |
| field keys | mono, 11px, `--text-dim`, letter-spacing 0.06em | none; prose only |
| what it reads as | a record with slots | a history |

A panel reads as a record; a rail reads as a history. Mono field keys also say
"these are three fields of one database row", not "three sentences somebody
wrote" — which is what stops state from being misread as the most recent
comment. Do not give the worklog a card, and do not give state a timeline.

## Status vocabulary

Six statuses. This is the whole state machine:

```
backlog → queue → active → review → done
                    ↓
                 blocked
```

`blocked` is **not a stage in the pipeline** — it is a side state that `active`
falls into and returns from. Never render the six as an equal-width progress
bar; that misrepresents the machine. Where the machine is drawn (the spec's
status reference), `blocked` hangs off `active` on a vertical stem.

Two transitions are reserved for the human and can never be made by an agent:
`backlog → queue` (only the human decides what gets worked on) and
`review → done` (only the human decides something is finished). A task sitting
in `review` is therefore **always waiting on the person looking at the screen**.

### Status treatment

Shape carries the position; luminance carries the demand. Only two statuses are
allowed to be bright — `review` and `blocked`. `backlog` and `done` are the
dimmest things on any screen.

Build one `StatusMark.vue` taking `status` as a prop. Layout is always
`display: inline-flex; align-items: center; gap: 7px; font-size: 12.5px`.

| status | glyph | glyph spec | label color |
|---|---|---|---|
| `backlog` | hollow circle | 9×9, `box-shadow: inset 0 0 0 1.5px #595d6c` | `#9397ab` |
| `queue` | hollow circle | 9×9, `inset 0 0 0 1.5px #9397ab` | `#cfd3e5` |
| `active` | filled circle | 9×9, `background: #968ae0` | `#e4e7f5` |
| `review` | filled circle + glow | 9×9, `background: var(--accent)`, `box-shadow: 0 0 0 3px color-mix(in srgb, var(--accent) 22%, transparent)` | `var(--accent)` |
| `done` | check | 10×10 SVG, 1.8px stroke, `stroke-linecap: round`, path `M2 6.4 4.6 9 10 3.2` on a 12×12 viewBox | `#75798c` |
| `blocked` | diamond | 8×8 square, `background: var(--blocked)`, `transform: rotate(45deg)` | `var(--blocked)` |

Read the system as: **circle = on the pipeline; check = left it; diamond = fell
off the side.** The glow on `review` is the only glow anywhere in the app —
spend it nowhere else.

The 120ms `--motion` transition applies to the mark's color and background, so
a transition the human just made is legible as a change rather than a repaint.

## Screens

### 1. Board — the root URL

The screen that gets opened many times a day. A cross-project table, one row per
task.

Columns, in this order, **all specified and not up for redesign**:

`project | task | status | next step | last updated by | how long ago`

Hard rules:
- Sorted most-recently-touched first. **Always.** No other sort.
- **Read-only** about tasks. No inline editing, no drag, no checkboxes, no bulk
  actions.
- **No filters.** No search box, no status tabs, no saved views.
- Clicking the project cell goes to the project. Clicking the row goes to the
  task.

**Layout.** Page frame is the nav bar (see below) over a body padded
`var(--s-6) var(--s-6) var(--s-8)`. Above the table, one header line, indented
14px to align with the table's first content column:

`8 tasks · [glow dot] 1 waiting on you` … `most recently touched first` …
`[New project]`

The left group is 12.5px `--text-dim`; the "waiting on you" segment is
`var(--accent)` with the same 7px glow dot as the review status mark. The
right-hand mono line is 12px `--text-faint`. `New project` is a ghost button
(see screen 5b).

**Table.** `table-layout: fixed`. Column widths go on the `<th>` cells — the
first row's widths define the columns:

| col | width | notes |
|---|---|---|
| edge mark | 14px | empty header; see review signal below |
| project | 110px | link, 12.5px, `--text-muted` |
| task | *remainder* (~470px at 1280) | link, 13px, `--text`; one-line ellipsis |
| status | 118px | `StatusMark` |
| next step | 330px | 13px `--text-muted`; one-line ellipsis |
| last updated by | 104px | actor, 12.5px |
| how long ago | 56px | right-aligned, `font-variant-numeric: tabular-nums` |

Header cells are 11px, uppercase, `letter-spacing: 0.08em`, 60%-alpha text.
Cell padding is `var(--s-2)` — rows land at 30px so a full board fits one
screen without scrolling. The five-second glance is the whole use case; do not
loosen this.

Row rules and the row hover tint are row-level background strips (see the rules
note in the tokens section), so the end-fade spans the row rather than each
cell. Hover tint is
`color-mix(in srgb, var(--text) 4%, transparent)` layered over the rule.

**Truncation.** Long values clip to one line with an ellipsis and carry the full
text in a `title` attribute. Never two lines — a row that can grow breaks the
scan rhythm, and the full text is one click away.

**"review = waiting on you", without a filter.** Three light touches, no
controls:
1. A **2px accent mark at the row's left edge** — an inset box-shadow on the
   14px edge-mark cell: `inset 2px 0 0 var(--accent)`. Survives peripheral
   vision at the far edge of a wide monitor.
2. The **count in the header line** — `1 waiting on you`. A readout, not a
   control: nothing to click, nothing to toggle.
3. The glow on the review status mark.

**Empty and substituted cells — three distinct cases, and the distinction is
deliberate:**

| case | cell shows | style |
|---|---|---|
| no state row at all (nobody has touched the task) | `no state yet` | 12px, `--text-faint` |
| state exists, `next_step` empty (typically `done`) | `—` | 13px, `--text-faint` |
| status is `blocked` | the `blocked_on` text | 13px, `color-mix(in srgb, var(--blocked) 76%, #75798c)`, one-line ellipsis, full text in `title` |

The first two are different on purpose: words for a **missing record**, a dash
for an **empty field**. A dash in the first case would read as an empty field
rather than a missing row.

The third is the one intentional deviation from "next step is
`state.next_step`": on a blocked task `next_step` is usually empty and
`blocked_on` holds the only sentence that matters, so the cell borrows the
column. Same column, same width, no header change — the hue is what says the
field is different. **This is the only cell in the app whose meaning depends on
the row's status; comment it.**

The `last updated by` cell also goes to a dim `—` when there is no state: no
state, no author.

**Recency and luminance.** Anything touched under an hour ago sits at
`--text-muted` in the actor and time cells; older rows drop to `--text-dim`.
Done rows drop their task title to `--text-muted` and their project link to
`--text-dim`. It costs nothing and the top of the board glows slightly.

### 2. Project detail

Reached by clicking a project cell. The board minus the project column, plus the
one thing the human can do here: add a task. Not a second configurable view —
same information, same rules, no filters.

**Header.** Breadcrumb (`board / binbirnet`, mono 12px), then the project name
at `--t-xl`, then a meta line: `3 tasks · [diamond] 1 blocked`. A `New task`
primary button sits right, baseline-aligned with the meta line.

**New-task row.** Opens **in place** at the top of the table area, not behind a
modal — a `--surface-raised` panel at `--r-md` holding a title input (15px), a
body textarea (min-height 64px, placeholder "Body — what the next reader needs
to know before they start"), then `Create in backlog` / `Cancel` and a mono
note: `new tasks always start in backlog`. It is a title and a body; it is
faster to type than to open.

**Table.** Identical to the board minus the project column: edge 14px, task
remainder, status 118px, next step 330px, by 104px, ago 56px.

### 3. Task detail — the screen that matters

Everything about one task, and the only place the human acts on it.

**Two-column grid**: content `minmax(0, 1fr)` and a 310px actions rail. Content
padded `var(--s-8) var(--s-8) 40px`; the rail padded
`var(--s-8) var(--s-8) var(--s-8) 0`.

**Ordering, top to bottom** (this ordering is a design decision — keep it):

1. **Breadcrumb** — `binbirnet / bnn-4`, mono 12px, project as a link.
2. **Status line** — the `StatusMark` plus, for `blocked`, a mono 11px
   `--text-faint` note: `since 22 Aug 10:31`.
3. **Title** — `--t-xl`, `max-width: 32ch`.
4. **Body** — `--t-md`, `#cfd3e5`, `max-width: 70ch`, `text-wrap: pretty`.
5. **The state panel.**
6. **The worklog.**

Actions live in the rail, not inline, so the reading order is
*what happened → what's next → what I can do*.

#### The state panel

A `--surface-raised` panel, `--r-lg`, padded `var(--s-6) var(--s-8) var(--s-8)`,
with `--e-1` **plus a 2px inset left mark in the status color**
(`inset 2px 0 0 var(--blocked)` on a blocked task, `var(--accent)` on active or
review, nothing on backlog/queue/done).

Header row: `State` (11px uppercase, `--text-muted`) on the left, and on the
right a mono 11.5px `--text-faint` legend: `one record · overwritten in place`.
That legend is doing real work — it is what tells a first-time reader this is
not a comment.

Fields, stacked with `gap: var(--s-6)`. Each is a mono 11px key
(`letter-spacing: 0.06em`, `--text-dim`) over a paragraph.

**Field order depends on status.** When the task is blocked, `blocked_on` is the
most important text on the screen and comes **first**, in its own box: padded
`var(--s-4) var(--s-6)`, `--r-md`, `background: var(--blocked-tint)`, key in
`var(--blocked)`, and the value at **16px `--text`** — larger than every other
field. Otherwise the order is `where_i_left_off`, `next_step`, and
`blocked_on` is absent.

Non-blocked field values are `--t-md`, `line-height: 1.55`, `#e4e7f5`,
`max-width: 70ch`, `text-wrap: pretty`.

Byline last, 12px `--text-faint`: `written by [actor] · 1d ago`, the time
carrying an absolute `title`.

**Empty state:** the panel stays, filled with its own absence — `State` header
over "No state yet. The first agent to work this task writes one; until then the
body is all there is." So the reader learns where state lives before there is
any.

#### The worklog

Heading row: `Worklog` at `--t-lg` left, mono 11.5px `--text-faint` right:
`28 entries · append-only`.

**A rail, not a list.** Wrapper is padded `padding-left: var(--s-3)` and paints
a 1px vertical gradient at x=6px, fading transparent 20px from each end:

```css
background: linear-gradient(to bottom,
  transparent, var(--rule) 20px,
  var(--rule) calc(100% - 20px), transparent) no-repeat 6px 0 / 1px 100%;
```

Each entry is `display: grid; grid-template-columns: 11px 1fr; gap: var(--s-6)`
with `padding-bottom: var(--s-8)`. The first column holds a marker: a 5×5 dot
at `margin: 6px 0 0 3px`, `#595d6c` normally, `#968ae0` when the entry carries
a status change, and a 7×7 rotated square in `var(--blocked)` when the change
was into `blocked`.

Entry content:
- **Meta line** (`gap: var(--s-3)`, wrapping): actor at 12.5px `#cfd3e5`; then,
  if the entry carried a status change, a chip — mono 11px, `padding: 2px 8px`,
  `--r-sm`, `background: color-mix(in srgb, var(--text) 6%, transparent)`,
  reading `queue → active` with the arrow at `--text-dim` and the destination
  status in its own color (a transition into `blocked` uses
  `background: var(--blocked-tint)` instead); then the timestamp pushed right
  with `margin-left: auto`, mono 11px `--text-faint`, format `20 Aug 09:12`.
- **`what_was_tried`** — 13.5px, `line-height: 1.55`, `#cfd3e5`,
  `max-width: 76ch`.
- **`outcome`** — same metrics, `#9397ab`. The luminance drop is the only thing
  separating the two; no labels, no icons.

**Oldest to newest** — newest at the bottom, as specified.

**Length treatment.** A task worked by an agent over a few days can hold 30
entries. Keep the **last five expanded** and collapse everything older into a
single row at the **top** of the rail (which is where the old entries are, given
the ordering): a chevron plus mono 12px `--text-dim` reading
`23 earlier entries · 4 – 19 Aug`, hovering to `var(--accent)`, expanding in
place. Because the collapse sits at the top, the recent entries are never
hidden — which is the requirement.

New entries appear without animation.

#### The actions rail

Three blocks separated by top-edge fading rules with `padding-top: var(--s-8)`.

1. **`Available now`** — the legal moves, as full-width buttons,
   `justify-content: flex-start`. The first is `btn-primary` (accent outline),
   the rest `btn-secondary` (divider outline). **The human sees only legal
   moves. Never a disabled grid of every status.** `GET /api/tasks/{id}`
   returns the list of statuses the current user may legally move it to — the
   buttons are driven by that array, not hardcoded per screen.

   For reference, the expected sets:

   | from | buttons |
   |---|---|
   | `backlog` | **Queue it** *(human only)* |
   | `queue` | **Start work** · Back to backlog |
   | `active` | **Send to review** · Mark blocked |
   | `review` | **Mark done** *(human only)* · Send back to active |
   | `blocked` | **Unblock — back to active** · Send to backlog |
   | `done` | Reopen to active |

   The actions row is the second thing the 120ms transition applies to.

2. **`This task`** — a `<dl>` on a two-column grid, 12.5px:
   `reference` (mono), `project` (link), `created` (relative, absolute
   `title`), `worklog` (entry count).

3. **Destructive** — `Edit title & body` and `Delete task` as ghost buttons at
   `--text-muted`. Both human-only.

#### Delete confirmation

Delete removes the worklog with it, and the dialog says so. `--surface-raised`
at `--r-lg`, `--e-2`, `width: min(440px, 100%)`.

Title: `Delete bnn-4?`. Body: "This removes the task, its state, and **all 28
worklog entries**. The worklog is append-only everywhere else in Cairn; this is
the only way it is ever destroyed, and it cannot be undone." Then: "Type
`bnn-4` to confirm." A mono input, then `Cancel` / `Delete permanently` —
the latter disabled until the reference matches exactly.

Project deletion reuses this dialog verbatim, with the count of tasks and
worklog entries it takes with it.

### 5b. New project

The board is read-only about *tasks*, not about *projects*, so the board header
line carries one creation affordance: a ghost `New project` button. It opens a
row **in place above the table**; the board stays visible behind it, same
pattern as the new-task row on project detail. Two inputs and a button do not
earn a route of their own.

Panel: `--surface-raised`, `--r-md`, padded `var(--s-6)`. A flex row holding
**Project name** (flex: 1, 15px) and **Reference prefix** (170px fixed, mono,
lowercase). Then `Create project` / `Cancel` and a right-aligned 12px
`--text-dim` note: "Tasks here will be numbered `bnn-1`, `bnn-2` … The prefix
cannot be changed afterwards."

**The prefix is derived, then editable.** Typing the name fills it: lowercase,
consonant-skeleton to three or four characters, deduplicated against existing
projects. `binbirnet → bnn`; `cairn → cairn` if it is already short enough. The
human can overwrite it before creating and **never after** — it goes into every
task reference the project will ever have (`bnn-4`), and it is the only value in
Cairn a human cannot change later. Validate: lowercase alphanumeric, 2–8 chars,
unique.

### 4. First-launch setup

Shown once, on a brand-new install. **This is a person naming themselves on
their own server** — not creating an account. No email field, no confirmation
step, no terms checkbox, no "create account" framing.

Left-aligned panel on the bare ground (`padding: 64px 56px`, min-height 340px,
content `max-width: 340px`), the cairn mark at 20px above it. Title
`Name yourself` at 22px, then: "This is your server. The name you pick here is
the one that appears next to every task you touch."

Username, password, `Start`.

Below, at 12.5px `--text-dim`, the sentence that has to be here because this is
the only chance to say it: "There is no reset email and never will be. If you
forget this password, stop the server and run `cairn --reset-password` on the
machine it runs on."

### 5. Login

Username and password. Nothing else. **No "remember me", no "forgot password"
link** — there is nothing behind that link.

Same panel geometry as first launch, content `max-width: 300px`. Cairn mark,
`cairn` at 22px, two fields, `Sign in`. Use
`autocomplete="username"` / `"current-password"`.

### 6. Agents and tokens

Where the human registers an agent (`claude`, `codex`) and gets an API token to
paste into that agent's MCP config.

Two-column grid: the list at `minmax(0, 1fr)`, a 340px register rail.

**The token reveal is the one place in the app that raises its voice**, because
it is the only screen where dismissing it loses something unrecoverable. Panel
at `--r-lg`, `background: color-mix(in srgb, var(--accent) 7%, var(--surface-raised))`,
`box-shadow: inset 0 0 0 1px var(--accent)` — the only accent outline on a
surface anywhere.

Warning triangle plus `Copy this now — it is shown once` in `var(--accent)`,
11px uppercase. Then the token on a `--bg` strip at `--r-md`: mono **16px**,
`--text` — the largest mono on any screen — with a `Copy` primary button pushed
right. Then, 13px `#cfd3e5`: "Cairn stores only a hash of it. Close this panel
without copying and the token is gone — the only way forward is to **issue a new
one** and revoke this. Paste it into `claude`'s MCP config as the bearer token
for `http://localhost:7777/mcp`."

The wording says *issue another*, never *recover*, because there is no recovery.

**Existing tokens table.** `table-layout: fixed`: agent 150px, token remainder
(mono, elided middle: `crn_7fJq…3uQg`), last used 100px (relative time, or
`never` at `--text-faint`), actions 90px right-aligned holding a
`Revoke` ghost button. A never-used token dims its whole row to `--text-dim`.

**Register rail.** A `Name` field (mono input, placeholder `claude`), a
`Create token` primary block button, and a 12px `--text-faint` note: "The name
is what appears in `last updated by` and on every worklog entry that agent
writes. Pick the one you will recognise a month from now."

### Nav bar

On every authenticated screen. `display: flex; align-items: center;
gap: var(--s-4); padding: var(--s-3) var(--s-6)`, with the fading rule as its
bottom edge (**not** a border).

Cairn mark (15px, `fill: var(--accent)`) + `cairn` wordmark at 18px, with
`margin-right: auto`. Then `Board` and `Agents` links at 14px inheriting text
color, going `var(--accent)` on hover and for `[aria-current="page"]`. Then a
36×36 icon button holding the theme moon at `--text-dim`, then the username at
12.5px `--text-dim`. The username is a label, not a menu — there is nothing
behind it.

### Empty states

Flush left, no centred illustration, no exclamation. Each says what is missing
and, where there is one, the single next move.

- **Board, no projects** — the only one that explains anything, because it is
  the only one a person sees before they understand the product. "Nothing here
  yet." then "A project holds tasks; a task holds a note for whoever picks it
  up. Make one, or register an agent and let it file the first one over MCP."
  Buttons: `New project` · `Register an agent`.
- **Project, no tasks** — "No tasks in binbirnet." / "Everything you file starts
  in backlog and waits for you to queue it." Button: `New task`.
- **Task, no state** — the state panel stays, holding its own absence. See the
  state-panel section.

## Actor display: human vs agent

`last updated by` and every worklog byline is a name — `alper` (the human) or
`claude` / `codex` (agents). **Agents get a small glyph before the name; the
human gets nothing.**

The glyph is a chip: an 11×11 inline SVG, `stroke: currentColor`,
`stroke-width: 1.5`, `opacity: 0.8`, on a 16×16 viewBox — a rounded 8.8×8.8
square with eight 2.2px pins on the four sides. Layout is
`display: inline-flex; align-items: center; gap: 5px`.

Build one `ActorName.vue` taking `name` and `isAgent`. The distinction is
informational, not hierarchical: agents are first-class users here, so the glyph
is quiet — no color difference, no badge, no capitalisation change.

## Relative-time convention

One unit, always. **Never `1d 4h`**, and never the word "ago" in a table column
— the header says it once.

| elapsed | shown |
|---|---|
| < 60s | `now` |
| < 60m | `4m` · `22m` |
| < 24h | `2h` · `19h` |
| < 7d | `1d` · `6d` |
| < 4w | `1w` · `3w` |
| this year | `9 Aug` |
| older | `9 Aug 25` |

- **Truncate, do not round.** 59 minutes is `59m`; 61 minutes is `1h`.
- **Weeks stop at four.** Past that a relative figure stops carrying information
  and an absolute date carries more — and a board whose bottom rows read
  `9 Aug` instead of `3w` makes staleness visible rather than arithmetic.
- Right-aligned with `font-variant-numeric: tabular-nums`, so the column lines
  up and the eye can run down it.
- Every cell carries the full local timestamp in a `title` attribute.
- Worklog entries use absolute times instead: `20 Aug 09:12`. The state byline
  uses relative.

Implement as one `useRelativeTime` composable / `formatRelative(date)` helper.
No date library — this is seven branches and a supply-chain question.

## Interactions & behavior

- **Navigation.** Board row → task detail. Board project cell → project detail
  (stop propagation). Breadcrumbs back up. Nav links to board and agents.
- **Status transitions.** POST the chosen status; the response returns the new
  legal-move list and a new worklog entry. Re-render the mark and the actions
  block through the 120ms transition. **No success toast** — the status mark
  changing *is* the feedback.
- **Copy token.** Clipboard write, then the button label becomes `Copied` for
  ~1.5s. No toast.
- **Delete.** Type-to-confirm; the primary button is disabled until the typed
  reference matches exactly.
- **Worklog collapse.** Local component state, expands in place, no animation.
- **Theme toggle.** Cycles system → light → dark, persisted in
  `localStorage`, applied as `data-theme` on `<html>`. Wire it now even
  though light is unstyled.
- **Errors.** Inline, next to the thing that failed, in the blocked hue. There
  is no notification system and there is no one to notify.
- **Loading.** The board is a local SQLite read behind a same-origin JSON call;
  it is fast. Render nothing rather than a skeleton, and show a message only if
  the request actually fails.
- **Responsive.** Desktop-first. Below ~1100px let the task-detail actions rail
  drop under the content and let the board's `next step` column go first. It
  must not break; it does not need to be good.

## State management

Small enough that Pinia is optional; two composables would do.

- `board`: the task list from `GET /api/board` (each task with its state
  joined). Sorted server-side; do not re-sort.
- `task`: from `GET /api/tasks/{id}` — the task, its state, its worklog, and
  **the list of statuses the current user may legally move it to**. The actions
  rail renders from that array. Never derive legal moves on the client.
- `session`: username, authenticated flag.
- `agents`: the token list. The plaintext token exists only in the component
  that received the create response — never persist it, never put it in a store
  that survives navigation.
- UI-local: new-task open, new-project open, worklog-expanded, delete-dialog
  open, theme preference.

## API

JSON, same-origin, already returns everything these screens need.

- `GET /api/board` → each task with its state joined.
- `GET /api/tasks/{id}` → the task, its state, its worklog, and the legal
  transitions for the current user.

The Vue app lives in `web/` and is embedded via `embed.FS`.

## Suggested component structure

```
web/src/
  styles/tokens.css          the token sheet above, imported once
  components/
    NavBar.vue
    StatusMark.vue           status → glyph + label
    ActorName.vue            name + agent chip glyph
    RelativeTime.vue         value + absolute title
    StatePanel.vue           the panel; blocked_on promotion lives here
    Worklog.vue              the rail
    WorklogEntry.vue
    TransitionActions.vue    renders from the legal-moves array
    TaskTable.vue            board + project detail share this
    ConfirmDialog.vue        task and project deletion
    icons/                   the eleven inline SVGs
  views/
    BoardView.vue
    ProjectView.vue
    TaskView.vue
    AgentsView.vue
    LoginView.vue
    SetupView.vue
  composables/
    useRelativeTime.js
    useTheme.js
```

`TaskTable.vue` taking a `showProject` prop covers both the board and project
detail; they are the same table.

## Sample data

Use this rather than lorem ipsum. The real text is long and awkward and that is
the point — several of the decisions above (truncation, the 76ch worklog
measure, the blocked_on promotion) only make sense against it.

**Board:**

| project | task | status | next step | by | ago |
|---|---|---|---|---|---|
| cairn | Embed the frontend into the binary | review | check the binary serves / with no dist on disk | claude | 4m |
| cairn | MCP server at /mcp over streamable HTTP | active | wire the transition tool to the service layer | claude | 22m |
| cairn | Decide SQLite vs Postgres | done | — | alper | 2h |
| binbirnet | Migrate the old invoice importer | blocked | *(blocked_on)* | codex | 1d |
| binbirnet | Nightly export job silently drops rows when the SFTP handshake times out | active | retry with backoff and log the handshake failure instead of swallowing it, then confirm against last week's dropped batch | codex | 6d |
| cairn | --reset-password should revoke sessions | backlog | *(no state)* | — | 3d |
| cairn | Vendor the Inter subset instead of the CDN link | queue | *(no state)* | — | 1w |
| binbirnet | Postgres connection pool exhausted under the importer | done | — | alper | 9 Aug |

**State block, full length, from the blocked task (`bnn-4`):**

- `where_i_left_off`: "The importer parses the 2019 format but chokes on the
  2021 header row, which added a currency column in the middle rather than at
  the end. Rewrote the column mapping to be name-based instead of positional."
- `next_step`: "Get a sample file from the 2022 exports and confirm the header
  names did not change again before finishing the mapper."
- `blocked_on`: "Need read access to the archive bucket — the 2022 exports are
  not in the repo."
- Written by `codex`, 1d ago.

**A worklog entry:**

- `what_was_tried`: "Pointed embed.FS at web/dist and ran go build."
- `outcome`: "Failed: pattern all:dist matches no files, because the frontend
  build had not run yet. Committed a placeholder so a clean checkout builds."
- `claude · active → blocked · 22 Aug 14:03`

The spec page's task-detail section carries five more entries for `bnn-4` in the
same register, plus the collapsed `23 earlier entries` row.

## Do not build these

**This list is closed, not a backlog.** Proposing or scaffolding any of these is
a miss, not an idea:

Gantt · calendar view · per-project table views or saved views · due dates ·
reminders · recurring tasks · labels · tags · priorities · estimates · sprints ·
cycles · milestones · file attachments · comments · subtasks · task hierarchy ·
dependencies · CalDAV · SMTP · notifications · toasts for routine success ·
mobile app · desktop app · multi-user anything: teams, roles, invitations,
sharing, assignee pickers, mentions, activity feeds of other people.

There is one user. There is no one to notify, no one to assign to, and no one to
share with.

## Assets

No external assets. No images, no photographs, no icon fonts, no webfonts
fetched at runtime. The eleven icons are inline SVG, listed in the Icons section
above and extractable verbatim from the spec page. If Inter is vendored it must
be a local woff2 subset committed to the repo.

## Files in this bundle

- `Cairn - interface spec.dc.html` — the design reference. Open it in a browser
  and read it alongside this README; every section is anchored. It carries the
  design notes inline, which sometimes explain *why* a value is what it is.
- `_ds/nocturne-.../styles.css` — the Nocturne design system stylesheet the
  spec page composes (`.btn`, `.input`, `.table`, `.card`, `.dialog`, `.nav`,
  `.tag`, `.field`). Useful as a reference for the component layer, but **do
  not vendor it into the Vue app** — build the handful of classes this product
  actually needs against `tokens.css` instead. The whole app is six components
  and four form controls; a design-system stylesheet is more surface area than
  the binary should carry.
- `support.js` — the spec page's runtime. Not part of the deliverable.

## The premise, so nothing drifts

Existing trackers are designed human-first and bolt agent access on afterwards
as a third-party add-on. Cairn does the opposite: agent behaviour is baked into
the schema and the rules. That premise has to be **visible in the interface**,
not just in the README — which is why agents appear as named actors with their
own glyph, why the worklog is a first-class record rather than a comment
thread, and why the two human-only transitions are the only asymmetry in the
state machine.

The name is the product thesis. A cairn is a stack of stones left on a trail so
whoever comes next knows the way. Every task carries a note for whoever picks it
up, human or agent.
