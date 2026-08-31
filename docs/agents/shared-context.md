# Shared context

The other half of every prompt in `in/`. Read this first; it is your brief, not
background reading.

## What Cairn is

An open-source issue tracker that treats a coding agent as a first-class user
rather than an API bolted onto a human tool. Apache-2.0, self-hosted, ships as
one Go binary with a Vue 3 frontend embedded in it via `embed.FS`. SQLite in WAL
mode, one file, no other runtime dependency. The repository is the working
directory you are in.

## The product thesis

A cairn is a stack of stones left on a trail so whoever comes next knows the
way. Every task carries a note for whoever picks it up, human or agent.

Every task has two records that do different jobs:

| | |
|---|---|
| **state** | one note, overwritten in place, always current. Three fields: `where_i_left_off`, `next_step`, `blocked_on`. Exactly one per task, enforced by the primary key. |
| **worklog** | append-only. `what_was_tried`, `outcome`, actor, time, and the status change it accompanied. Never edited, never deleted. This is where the dead ends live, so the next agent does not repeat them. |

An agent cannot move a task without writing state. This is enforced in the type
system, not by convention: the only function that writes `task.status` takes the
note as an argument, and status, state and worklog are written in one
transaction or not at all.

## The workflow

```
backlog -> queue -> active -> review -> done
                      |
                   blocked
```

`blocked` is a side state that `active` falls into and returns from, not a
pipeline stage. Two transitions are the human's alone and are refused to agents:
`backlog -> queue` (only the human decides what gets worked on) and
`review -> done` (only the human decides something is finished). So a task in
`review` is always waiting on the human — the single most actionable signal in
the product.

Beyond permission, some moves demand a payload (`internal/workflow.Requires`):
moving to `blocked` requires `blocked_on`; `review -> active` requires
`next_step`, because rejecting work without saying why leaves the agent
re-reading its own note; leaving `active` requires `what_was_tried`, because
what is not written down at that moment is never written down.

## User model

One human. N agents. No teams, roles, invitations, sharing, or assignees —
there is nobody to share with. The human authenticates with a password
(argon2id, 64MB, 3 passes) and gets a session cookie; agents authenticate with
bearer tokens stored as sha256 plus a short prefix, shown once at issue time.

## Layout

```
internal/workflow/   the state machine. stdlib only. pure data, pure functions.
internal/model/      the types that cross layer boundaries
internal/store/      SQL only, no rules
internal/service/    every rule; the only package that writes
internal/httpapi/    JSON API for the browser (net/http mux, Go 1.22 patterns)
internal/mcpserver/  MCP tools for agents, over streamable HTTP at /mcp
web/                 Vue 3 + Vite app, embedded into the binary
site/                the static marketing site, published to GitHub Pages
docs/design-brief.md the brief the interface was designed from
docs/agents/         these prompts, and the reports they produce
```

Two tests guard the architecture rather than any behaviour, and both must keep
passing: `TestOnlyServiceReachesTheStore` parses every Go file and fails if
anything outside `internal/service` imports `internal/store`;
`TestEveryServiceMethodDecidesWhoMayCallIt` reflects over the service and fails
if a method has not declared which operation it requires.

The eight MCP tools: `board`, `list_projects`, `list_tasks`, `get_task`,
`create_task`, `transition_task`, `write_state`, `append_worklog`. Every task
read includes `can_move_to`, so an agent never guesses a transition and burns a
turn being refused.

## Hard constraints

These are settled. Proposing against them is a miss, not an idea.

1. **The out-of-scope list is closed, not a backlog.** Do not propose, and do not
   design around: Gantt, calendar view, saved views, due dates, reminders,
   recurring tasks, labels, tags, priorities, estimates, sprints, cycles,
   milestones, attachments, comments, subtasks, task hierarchy, dependencies,
   CalDAV, SMTP, notifications, toasts for routine success, mobile or desktop
   apps, or multi-user anything.
2. **It must work fully offline and airgapped.** No CDN, no Google Fonts, no
   runtime fetch of anything, no telemetry, no phone-home.
3. **Dependencies are a cost** paid by every user of a security-adjacent
   self-hosted tool. The frontend has exactly two runtime dependencies (`vue`,
   `vue-router`) and no CSS framework or icon library. Adding one needs an
   argument, not a preference.
4. **One human, N agents.** Anything that only makes sense with a second person
   is out.
5. **The interface never derives legal moves.** It renders whatever
   `can_move_to` says, so the state machine has exactly one implementation.
6. **The workflow package stays free** of database, HTTP and service types.

## House style

Read the existing code and prose before writing any. Comments here explain why a
decision was made, not what a line does, and the error strings are written to be
read by an agent the way a person reads documentation. Match that. Prose in this
project does not hedge, does not use marketing register, and does not announce
its own cleverness.

## How to report

Write your report to the path your prompt names, in `docs/agents/out/`. If a
report is already there, read it first and write one that says what changed
rather than repeating it.

Begin the file with this block, filled in:

```markdown
---
role:    <your role>
date:    <today>
commit:  <output of `git rev-parse --short HEAD`>
scope:   <what you actually read, honestly — say if you skipped something>
---
```

Then follow the shape your prompt asks for. Three rules hold for every role:

- **Evidence, not opinion.** Every finding cites `file:line` and states a
  concrete failure scenario: inputs or sequence, and the wrong result they
  produce. "This could be clearer" is not a finding.
- **Rank, and be willing to be wrong about the ranking.** A list of ten equally
  confident findings has told the reader nothing about which to read first.
- **End with what you are least sure about.** That section is not a disclaimer;
  it is usually where the real problem is.

Do not edit files in `out/` that belong to another role, and do not act on
another role's findings — read them for context, and say where you disagree.

If Cairn's own MCP server is connected, you may also file findings as tasks in
project `cairn`. They land in `backlog`, which is correct: you proposed the work,
the human decides what gets done.
