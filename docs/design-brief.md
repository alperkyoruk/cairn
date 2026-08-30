# Cairn — design brief

A brief for designing the web interface. Everything here is settled; the open
questions are visual, not functional.

## What Cairn is

A self-hosted issue tracker that treats a coding agent as a first-class user.
Ships as a single Go binary with the frontend embedded in it. Apache-2.0.

Existing trackers are designed human-first and bolt agent access on afterwards
as a third-party add-on. Cairn does the opposite: agent behaviour is baked into
the schema and the rules. That premise has to be visible in the interface, not
just in the README.

The name is the product thesis. A cairn is a stack of stones left on a trail so
whoever comes next knows the way. Every task carries a note for whoever picks it
up, human or agent.

## Who is looking at this

One person. Their own machine or their own server. There is no team, no
invitations, no sharing, no permissions UI, no avatars-of-your-colleagues.

The other users are coding agents, and **they never see this interface** — they
talk to the same server over MCP. So the UI's job is not to help an agent work.
Its job is to let one human see, at a glance, what their agents have been doing
and what needs a decision from them.

Assume this person opens the root URL many times a day for five seconds at a
time. Density and scannability beat spaciousness.

## The one concept the interface has to make legible

Every task carries two separate records, and a reader must never confuse them:

**state** — a single summary, overwritten in place, always current. Three fields:
`where_i_left_off`, `next_step`, `blocked_on`. Plus who wrote it and when.
There is exactly one of these per task. It is the note on top of the cairn.

**worklog** — append-only. Never edited, never deleted. Each entry:
`what_was_tried`, `outcome`, who, when, and the status change it accompanied if
there was one. This is the trail behind the cairn.

The design problem: state is small, current, and the thing you read first.
Worklog is long, historical, and the thing you read when you need to know how
the task got here. They should not look like two lists of the same kind of
thing, and state must not read as "the most recent comment."

## Screens

### 1. Board — the root URL

A cross-project table. One row is one task. This is the screen that gets opened
many times a day.

Columns, in this order, all specified and not up for redesign:

| project | task | status | next step | last updated by | how long ago |

- Sorted most-recently-touched first. Always. No other sort.
- **Read-only.** No inline editing, no drag, no checkboxes, no bulk actions.
- **No filters.** No search box, no status tabs, no saved views.
- Clicking the project cell goes to the project. Clicking the row goes to the task.
- "next step" is `state.next_step`. It may be empty — a task nobody has touched
  yet has no state at all. That empty case needs a deliberate treatment; it is
  common, not an edge case.
- "last updated by" is a name, human or agent, e.g. `alper` or `claude`. Whether
  agents are visually distinguished from the human here is a design decision
  worth making explicitly.
- "how long ago" is relative: `4m`, `2h`, `3d`. Needs a convention.

### 2. Project detail

Reached by clicking a project. Shows the project's own tasks and lets the human
create one. Not a second configurable view of the board — just this project's
tasks, same information, minus the project column.

### 3. Task detail — the screen that matters

Everything about one task, and the only place the human acts on it.

Contains, and the ordering is a design decision:

- The task itself: reference (`cairn-12`), title, body, current status, project.
- **The state block** — `where_i_left_off`, `next_step`, and `blocked_on` when
  set, with who wrote it and when. When a task is blocked, `blocked_on` is the
  most important text on the screen.
- **The actions available**, which depend on status. The human sees only legal
  moves: from `review` the choices are *send back to active* or *mark done*.
  From `queue`, *start work*. And so on. Never a disabled grid of every status.
- **The worklog**, oldest to newest, each entry showing actor, time, what was
  tried, outcome, and the status change if any. This gets long — a task worked
  by an agent over a few days can hold 30 entries. Needs a treatment that stays
  readable at that length without hiding the recent ones.
- Editing title and body, and deleting the task. Both human-only. Delete removes
  the worklog with it, which is worth a confirmation that says so.

### 4. First-launch setup

Shown once, on a brand-new install: pick a username and a password. No email
field, no confirmation step, no terms checkbox, no "create account" framing —
this is a person naming themselves on their own server. Worth saying somewhere
that the only recovery path is a `--reset-password` flag on the command line,
because there is no reset email and never will be.

### 5. Login

Username and password. Nothing else. No "remember me", no "forgot password"
link — there is nothing behind that link.

### 6. Agents and tokens

Where the human registers an agent (`claude`, `codex`) and gets an API token to
paste into that agent's MCP config. The token is shown exactly once and stored
only as a hash; that needs to be unmissable, because there is no way to recover
it — only to issue another. Also lists existing tokens with last-used time, and
revokes them.

## The status vocabulary

Six statuses. This is the whole state machine:

```
backlog → queue → active → review → done
                    ↓
                 blocked
```

`blocked` is not a stage in the pipeline — it is a side state that `active`
falls into and returns from. Colour and placement should reflect that; laying
the six out as an equal-width progress bar would misrepresent it.

Two transitions are reserved for the human and can never be made by an agent:
`backlog → queue` (only the human decides what gets worked on) and
`review → done` (only the human decides something is finished). A task sitting
in `review` is therefore always waiting on the person looking at the screen.
That is arguably the single most actionable signal in the product, and the board
currently does nothing to surface it. Open to ideas that don't require a filter.

## Sample data to design against

Please don't design against lorem ipsum — the real text is long and awkward and
that is the point.

| project | task | status | next step | by | ago |
|---|---|---|---|---|---|
| cairn | Embed the frontend into the binary | review | check the binary serves / with no dist on disk | claude | 4m |
| cairn | MCP server at /mcp over streamable HTTP | active | wire the transition tool to the service layer | claude | 22m |
| cairn | Decide SQLite vs Postgres | done | — | alper | 2h |
| binbirnet | Migrate the old invoice importer | blocked | — | codex | 1d |
| cairn | --reset-password should revoke sessions | backlog | | | 3d |

A state block, full length, from a blocked task:

- **where_i_left_off**: "The importer parses the 2019 format but chokes on the
  2021 header row, which added a currency column in the middle rather than at
  the end. Rewrote the column mapping to be name-based instead of positional."
- **next_step**: "Get a sample file from the 2022 exports and confirm the header
  names did not change again before finishing the mapper."
- **blocked_on**: "Need read access to the archive bucket — the 2022 exports are
  not in the repo."

A worklog entry:

- **what_was_tried**: "Pointed embed.FS at web/dist and ran go build."
- **outcome**: "Failed: pattern all:dist matches no files, because the frontend
  build had not run yet. Committed a placeholder so a clean checkout builds."
- *claude · active → blocked · 22 Aug 14:03*

## Technical constraints

These are hard.

- **Vue 3** with Vite. Composition API, `<script setup>`.
- **Must work completely offline.** The whole app is embedded in a single binary
  that people run on their own machines, sometimes airgapped. No Google Fonts,
  no CDN, no runtime fetch of anything. Any font, icon, or asset must be
  vendorable into the repo — a system font stack is very welcome.
- **Light and dark**, following `prefers-color-scheme`, with an explicit toggle.
- Desktop-first. It should not break on a narrow window, but there is no mobile
  app and phone use is not a goal.
- Prefer no CSS framework and no icon library. Plain CSS with custom properties
  is ideal; a handful of inline SVGs is fine. Every dependency ends up in the
  binary and in the supply chain of a security-adjacent self-hosted tool.
- No animation beyond what makes a state change legible.

## Do not design these

This list is closed, not a backlog. Proposing any of these is a miss, not an
idea:

Gantt · calendar view · per-project table views or saved views · due dates ·
reminders · recurring tasks · labels · tags · priorities · estimates · sprints ·
cycles · milestones · file attachments · comments · subtasks · task hierarchy ·
dependencies · CalDAV · SMTP · notifications · toasts for routine success ·
mobile app · desktop app · multi-user anything: teams, roles, invitations,
sharing, assignee pickers, mentions, activity feeds of other people.

There is one user. There is no one to notify, no one to assign to, and no one to
share with.

## What would be most useful back

1. **Design tokens** as CSS custom properties — colour (light + dark), type
   scale, spacing scale, radii, borders. This is the highest-value deliverable;
   everything else can be derived from it.
2. **Status treatment** — how the six statuses read as colour and shape, with
   `blocked` distinct from the linear four and `review` legible as "waiting on
   you".
3. **Task detail**, laid out. The state block versus the worklog is the real
   design problem in this product.
4. **Board**, at desktop width, including the empty `next_step` case and how a
   long `next_step` truncates.
5. Empty states: no projects yet, a project with no tasks, a task with no state.
6. A relative-time convention (`4m` / `2h` / `3d` — and what happens past a week).

## Where this lands

The Vue app lives in `web/` and is embedded via `embed.FS`. The HTTP API is
JSON, same-origin, and already returns everything these screens need:
`GET /api/board` returns each task with its state joined; `GET /api/tasks/{id}`
returns the task, its state, its worklog, and the list of statuses the current
user may legally move it to — so the actions on task detail are driven by data,
not hardcoded per screen.
