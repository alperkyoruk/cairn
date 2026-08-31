---
role:    product manager
date:    2026-08-31
commit:  dfee902
scope:   README.md, docs/design-brief.md, internal/workflow/workflow.go,
         internal/service/task.go, internal/store/task.go, internal/httpapi/dto.go,
         internal/mcpserver/{tools.go,mcpserver.go}, all seven views in
         web/src/views/, the components they use, web/src/{router.js,api.js}, and
         the git history behind the routing and ordering decisions. Did not read
         auth.go beyond agent creation, the store tests, or site/.
filed:   cairn-1 … cairn-9, all in backlog. cairn-7 and cairn-8 were cut from
         this report for length and exist only as tasks.
---

# What the product is for

Cairn is where one person parks the intent behind work that will be done while
they are not watching, and finds out on return whether it was finished,
abandoned, or stuck. The two records exist so an agent's context outlives the
agent: `state` is what the next reader needs, `worklog` is what they must not
repeat. Everything else is scaffolding around the moment a task changes hands.

The README calls the agent a first-class *actor*. Read as code it is a
first-class **writer** and the human a reader: every obligation in
`workflow.Requires` and every check in `Transition` points at the agent, and the
one thing the human must write is destroyed on the agent's next write.

# The two loops, and where they leak

**The human loop** — open the board → notice → act → close the tab.

- The root URL is not the board. `router.js:12` maps `/` to `ProjectsView`, where
  "what needs a decision from me" is answered only as a count
  (`ProjectsView.vue:160`); the rows are a nav away (`:108` → `/tasks`). The
  five-second glance spends its first second on a page load.
- On `/tasks`, `review` is not ordered by actionability. `store/task.go:285`
  sorts done-last then `updated_at DESC`, so a review task waiting three days
  sits below an active task touched four minutes ago. The 2px edge mark
  (`TaskTable.vue:154`) only helps rows already in the viewport.
- "Is anything stuck?" has no answer for the commonest kind of stuck. `blocked`
  is handled well — `TaskTable.vue:29-37` borrows the next-step column for
  `blocked_on`. But a task in `active` whose agent died looks identical to one
  being worked right now, bar the text in the last column; `TaskTable.vue:41-43`
  is the only staleness signal, with a one-hour threshold that dims two cells
  (`:156`). The human holds "how long is too long for active" in their head,
  fifteen times a day. The README names this failure — "a lie the board tells you
  every time you open it" — and answers it with prose for `CLAUDE.md`.

**The agent loop** — pick up → work → leave an honest record → stop.

- `board` and `list_tasks` return `taskOut` (`mcpserver/tools.go:68-76`), which
  has no `blocked_on`: an agent sees a blocked task and a `next_step` written
  before the block, and must spend a turn on `get_task` to learn the blocker.
  The human's board solved this; the agent's did not.
- The connect instructions promise "Every task read includes can_move_to"
  (`mcpserver.go:57`). `taskOut` has none, so an agent scanning fifty rows for
  claimable work either guesses or burns fifty calls.
- Everything is enforced at write time; nothing watches for the *absence* of a
  write, which is what "an agent stopped" looks like.

# Three improvements that add no feature

**1. Make the landing screen answer the question.** Float `review` above
everything open, exactly as `done` sinks below it — one more `CASE` arm in
`store/task.go:285`. The argument is already in the tree: `ae54f05` sank done
because recency alone floated fresh completed work above the thing you are stuck
on; review is that mistake in the other direction, and doing it in SQL means the
MCP board inherits it. It only pays off if the board is what `/` shows:
`5682c25` optimised the first visit against the next five thousand. Swap the
routes back, or put the review *rows* on the root. **Size:** one line of SQL.

**2. Stop the human's rejection erasing the agent's account of its own work.**
`service/task.go:220-222` rejects any state write with an empty
`where_i_left_off`, so `TaskView.vue:30-36` makes the human fill it, and
`store.UpsertState` (`store/task.go:127-137`) overwrites the column wholesale.
The agent writes what it built; the human sends it back with *"the 2022 headers
still fail"*; the agent picks the task up reading a symptom where its own record
was. `workflow.Requires` (`workflow.go:197`) only ever demanded `next_step` for
that move. Require only that, leave `where_i_left_off` alone, and write the
human's finding to the worklog via `api.js:58` — defined already, called by
nothing. **Size:** one relaxed condition, one form field, one worklog write.

**3. Give the agent's board what the human's board has.** Add `blocked_on` and
`can_move_to` to `taskOut`. `NextFor` is a pure map lookup, so per-row
`can_move_to` costs no query. It also makes `mcpserver.go:57` true. **Size:**
three fields and a loop.

# New capabilities

**A. `active` staleness as a readout.** A task in `active` with no write for
longer than a threshold is marked, and its "ago" reads as *silence* rather than
recency. — *One human:* yes; it describes agents, not people. *Offline:* yes; it
compares `updated_at` to the clock, on data the row already carries. *Not on the
closed list:* not a **due date** — no date is entered, nothing is scheduled; not
a **reminder** or **notification** — nothing is delivered, nothing fires while
you are away. The only proposal that answers "is anything stuck?" for the case
the product cannot see.

**B. "Since you last looked."** Stamp the human's last board read on the actor
row; mark tasks touched since. — *One human:* yes, and only with one: "my last
visit" is meaningless the moment there are two of us. *Offline:* yes; one column,
one write. *Not on the closed list:* not an **activity feed of other people** —
there are none, and it adds no screen, only a mark on existing rows; not a
**notification** — it exists only while you are looking. At fifteen opens a day
its usual answer is "nothing", in under a second.

I stopped at two. The third — a per-agent view of what each holds in `active` —
is an assignee picker in a hat.

# The unflattering thing

The README's charge against every other tracker is that the interesting part
"ends up as prose in a comment box, if it is recorded at all." Cairn does that to
the human. Every sentence the human writes in the shipped interface goes to
`state`, the record designed to be overwritten: `api.js:57-58` define
`writeState` and `appendWorklog` and nothing calls either, and `TaskView.vue:97`
sends `worklog: null` on every human transition. The append-only record — the
thing the product is named after — holds nothing the human wrote except status
arrows. Run this six months across three repositories and the worklog is a
complete account of what your agents did and a blank on why you kept sending
things back. The README presents the asymmetry as a virtue ("the human's
own obligation") without noticing it is discharged into a field guaranteed to be
erased.

# What I am least sure about

The ordering change in #1. A board whose order is a fact is trustworthy in a way
one with priority bands is not, and `done` is a weaker precedent than I made it
sound: done is terminal, review is not. If the real habit is "scan the top five",
floating review could push an urgent blocked task out of the glance.

Whether `/` should be the board at all. `5682c25` is a better argument than I
credited, and its author has used this daily; I read it for an afternoon.

Capability A's threshold. I have no data on a normal `active` stretch and
deliberately proposed no number. A threshold that fires wrongly is worse than no
mark, which is the whole risk in it.
