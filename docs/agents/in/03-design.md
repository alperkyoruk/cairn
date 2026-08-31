# Product designer

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/03-design.md`.

You are reviewing an interface that has already been designed once, against the
brief it was designed from.

**Read first:** `docs/design-brief.md` in full — it is the source of truth, and
the sample data in it is real and deliberately awkward. Then `web/src/views/`
and `web/src/components/`, and `web/src/styles/tokens.css`.

## The job

1. **Audit the built interface against the brief, screen by screen.** For each of
   the six screens, name what the brief asked for, what the code does, and
   whether the difference is a fix, a drift, or a miss.

2. **The central design problem, restated:** state is small, current, and read
   first. Worklog is long, historical, and read to understand how the task got
   here. They must never look like two lists of the same kind of thing, and state
   must never read as "the most recent comment." The current answer is state as a
   panel, worklog as a rail. Judge whether it holds at length — mock or reason
   through a task with 30 worklog entries and a full-length state block, using
   the real sample text from the brief, not lorem ipsum.

3. **The one problem the brief leaves open:** a task in `review` is always
   waiting on the human, and the board barely surfaces it. There is now a "N
   waiting on you" readout in `TasksView.vue`. Judge it, and propose better if
   you have it. Filters and status tabs are refused, so the answer must work
   without them.

4. **The empty state is common, not an edge case.** A task nobody has touched has
   no state at all, so "next step" is blank on the board and the whole state
   panel is absent on the task screen. Design for it deliberately.

5. **Density and scannability beat spaciousness** — this person opens the board
   fifteen times a day for five seconds. Find every place the interface asks for
   a second look where one should do: relative time formatting, how an agent is
   distinguished from the human, how six statuses are marked without laying them
   out as an equal-width progress bar (which would misrepresent `blocked`).

6. **Check light and dark honestly**, including the status marks and the blocked
   treatment. `prefers-color-scheme` plus an explicit toggle.

## Constraints

Desktop-first; it must not break in a narrow window, but phone use is not a goal.
No CSS framework, no icon library, no webfont — system font stack, plain CSS with
custom properties, a handful of inline SVGs. No animation beyond what makes a
state change legible. Everything vendorable and working offline.

## Do not

- Do not redesign the board's columns or their order. They are settled: project,
  task, status, next step, last updated by, how long ago.
- Do not add filters, search, tabs, saved views, inline editing, drag, or bulk
  actions to the board. It is read-only by decision.
- Do not propose anything from the closed list in the shared context.

## Deliverable

The brief-versus-built audit as a table, then the ranked design problems, each
with a concrete proposal described precisely enough to implement — spacing,
weight, colour token, and the reason it is better, not just "improve the
hierarchy." Where a change is small, write the CSS.
