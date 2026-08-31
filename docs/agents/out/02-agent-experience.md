---
role:    agent-experience engineer
date:    2026-08-31
commit:  dfee902
scope:   internal/mcpserver/{mcpserver.go,tools.go,errors.go,mcpserver_test.go},
         internal/workflow/workflow.go, internal/service/{task.go,errors.go,service.go},
         internal/store/task.go (Board, ListTasksByProject, ListWorklog),
         web/src/components/{TaskTable.vue,StatePanel.vue}, README.md:286-318.
         I also called this Cairn's own MCP server as an agent and drove every
         refusal path below; the error strings quoted as "measured" are what came
         back, not what I read in the source. I did not read internal/httpapi
         beyond credential handling, the store tests, or site/.
filed:   cairn-10 … cairn-17, all in backlog.
prior:   docs/agents/out/01-product.md (product manager, same commit). I agree
         with cairn-4 and cairn-8 and extend both; where I disagree it is marked.
---

# The one-line version

The prose on this surface is better than the prose on any MCP server I have been
handed, and it fails in one direction consistently: it explains the rule at the
place the rule was written, and the agent meets the rule somewhere else. Every
finding below is a gap between those two places.

There is also one field an agent can fill, correctly, following its own
description, that no human will ever see. That one is not a prose problem.

# 1. String rewrites

The index first; the pairs and the patches follow.

| # | tool / site | what it fixes |
|---|---|---|
| R1 | `append_worklog.what_was_tried` | the only annotation that restates the field name, on the tool whose entire value is that one string |
| R2 | `append_worklog.outcome` | same column as `transition_task.outcome`, without the clause that asks for dead ends |
| R3 | `write_state.where_i_left_off` | thinner than the identical field on `transition_task`, on the tool used when the agent may be about to vanish |
| R4 | `write_state.blocked_on` | "if anything" invites the write-only trap in §5 |
| R5 | `transition_task.what_was_tried` | schema says optional, service requires it for the two commonest moves; the annotation buries that after a sentence |
| R6 | `list_tasks.status` | `board` names the six statuses, `list_tasks` does not |
| R7 | `transition_task.to` | the enum's refusal is the SDK validator's, not Cairn's; put the explanation where the enum is read |
| R8 | `summariseDetail`, empty `can_move_to` | the sentence an agent lands on at the end of the commonest dead end |
| R9 | `summariseDetail`, truncation line | states a fact where it could give an instruction |
| R10 | connect-time instructions | §4 |

## R1 — `append_worklog.what_was_tried` (tools.go:62)

> **current:** what you attempted

> **proposed:** what you actually did this stretch, in enough detail that
> someone about to repeat it would recognise it. The approach, not the intention

`transition_task.what_was_tried` (tools.go:49) describes the same database
column and names the judgement. Two annotations, one column, and the thin one is
on the tool an agent reaches for most often mid-run. Three words each way; it
buys the difference between "worked on the importer" and "parsed the 2022
headers with encoding/csv, which chokes on the quoted region column".

## R2 — `append_worklog.outcome` (tools.go:63)

> **current:** what happened as a result

> **proposed:** what happened, including failures worth not repeating

Verbatim from `transition_task.outcome` (tools.go:50). The worklog exists for
dead ends — the package comment says so, the instructions say so — and the
annotation on the dedicated worklog tool is the one that does not.

## R3 — `write_state.where_i_left_off` (tools.go:55)

> **current:** what has actually been done so far

> **proposed:** what has actually been done, in enough detail that someone who
> was not here can carry on

Again verbatim from the transition version (tools.go:46). `write_state` is the
checkpoint tool: it is called when the agent might not get another turn, which
is exactly when the note has to stand on its own. It carries the weaker sentence.

## R4 — `write_state.blocked_on` (tools.go:57)

> **current:** what is blocking progress, if anything; required while the task
> is blocked

> **proposed:** why the task is blocked. Only read while the task is in blocked:
> if you have hit a blocker, move the task to blocked with transition_task
> instead, because a blocker recorded here on an active task appears on no board

"If anything" reads as an invitation to fill the field whenever something is in
the way. Doing that produces the artefact in §5.1. This rewrite is half the fix;
the other half is a guard in the service, because a field this easy to fill
wrongly should not be left to prose alone.

## R5 — `transition_task.what_was_tried` (tools.go:49)

> **current:** what you attempted during this stretch of work. Required when
> leaving active for review or blocked, because that is the moment the attempt
> gets recorded or never does

> **proposed:** Required when leaving active, for review or for blocked: what
> you attempted during this stretch of work, including what did not work. That
> moment is when the attempt gets recorded or never does

Same words, reordered. The field is optional in the schema — deliberately, and
locked by mcpserver_test.go:238-241 — and mandatory in the service for the two
most common agent transitions there are. `blocked_on` on the same struct
(tools.go:48) already leads with "required when". A model skimming a seven-field
schema reads the first clause of each; here the first clause says the field is a
nice-to-have.

## R6 — `list_tasks.status` (projectIn.Status, tools.go:28)

> **current:** restrict to these statuses; omit to get everything except done

> **proposed:** restrict to these statuses: backlog, queue, active, review,
> blocked, done. Omit to get everything except done

`boardIn.Status` (tools.go:33) names the vocabulary. The identical field one
struct up does not, and `list_tasks` is the tool the README's own `CLAUDE.md`
snippet points an agent at (README.md:300, "project `myproject`"). Measured cost
of guessing:

```
board(status: ["in_progress"])
  -> unknown status "in_progress"; the statuses are backlog, queue, active,
     review, done, blocked
```

A good error, and a whole turn to learn six words the annotation could have
carried.

## R7 — `transition_task.to` (tools.go:45)

> **current:** the status to move it to: active, review, or blocked

> **proposed:** the status to move it to: active, review, or blocked. queue and
> done are missing from this list because they are the human's: a task in
> backlog waits for them to queue it, and a task you move to review waits for
> them to mark it done

The enum makes the human's two decisions unexpressible, which is the intent
(mcpserver_test.go:369-375) and which I am not proposing to change. The
consequence, measured:

```
transition_task(ref: "cairn-4", to: "queue", …)
  -> validating "arguments": validating root: validating /properties/to:
     enum: queue does not equal any of: [active review blocked]
```

That is the only string on this surface not written for its reader. The test
comment at mcpserver_test.go:379 already concedes the trade — "the description
still says why, since the error no longer can" — and the description does say
why, in its third paragraph, under a heading about refusals. A schema enum is
read at the point of use, by a model deciding what to put in a field. Put the
sentence there and the terse refusal stops being the first place an agent learns
this.

I considered the alternative — restore `queue` and `done` to the enum so
`workflow.TransitionError` can answer with "only the human decides what gets
worked on" (workflow.go:250), which is the best-written string in the codebase
and is currently unreachable from MCP. I do not propose it. The enum prevents
the call; the good string only consoles the agent after it has spent the turn.

## R8 — `summariseDetail` with an empty `can_move_to` (tools.go:344)

> **current:** There is nothing you can move it to from here.

> **proposed**, by status:
> - backlog — Nothing here is yours to move: only the human moves a task out of backlog, by queueing it.
> - review — Nothing here is yours to move: it is with the human now, waiting to be marked done.
> - done — This one is finished.

Three statuses hand an agent an empty `can_move_to` (workflow.go:78-89:
backlog, review, done) and the right response differs in each. See §3, T2 —
this is the highest-value sentence on the surface.

## R9 — `summariseDetail` truncation line (tools.go:336)

> **current:** . Showing the last %d of %d worklog entries.

> **proposed:** . Showing the last %d of %d worklog entries; the abandoned
> approaches are usually among the older ones — pass worklog_limit to read them.

Reasoning in §4.

## The patches

Purely textual, against `dfee902`.

```diff
--- a/internal/mcpserver/tools.go
+++ b/internal/mcpserver/tools.go
@@
 type projectIn struct {
 	Project string   `json:"project" jsonschema:"the project slug, like cairn"`
-	Status  []string `json:"status,omitempty" jsonschema:"restrict to these statuses; omit to get everything except done"`
+	Status  []string `json:"status,omitempty" jsonschema:"restrict to these statuses: backlog, queue, active, review, blocked, done. Omit to get everything except done"`
 	Limit   int      `json:"limit,omitempty" jsonschema:"maximum tasks to return; defaults to 50"`
 }
@@
 type transitionIn struct {
 	Ref           string `json:"ref" jsonschema:"the task to move, like cairn-12"`
-	To            string `json:"to" jsonschema:"the status to move it to: active, review, or blocked"`
-	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on"`
+	To            string `json:"to" jsonschema:"the status to move it to: active, review, or blocked. queue and done are missing from this list because they are the human's: a task in backlog waits for them to queue it, and a task you move to review waits for them to mark it done"`
+	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on. When you are claiming queued work, that is what you have read and understood; next_step is what you are about to do first"`
 	NextStep      string `json:"next_step" jsonschema:"the single next thing whoever picks this up should do"`
 	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"required when moving to blocked: exactly what you need in order to continue"`
-	WhatWasTried  string `json:"what_was_tried,omitempty" jsonschema:"what you attempted during this stretch of work. Required when leaving active for review or blocked, because that is the moment the attempt gets recorded or never does"`
+	WhatWasTried  string `json:"what_was_tried,omitempty" jsonschema:"Required when leaving active, for review or for blocked: what you attempted during this stretch of work, including what did not work. That moment is when the attempt gets recorded or never does"`
 	Outcome       string `json:"outcome,omitempty" jsonschema:"what happened, including failures worth not repeating"`
 }
@@
 type writeStateIn struct {
 	Ref           string `json:"ref" jsonschema:"the task to leave a note on, like cairn-12"`
-	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done so far"`
+	WhereILeftOff string `json:"where_i_left_off" jsonschema:"what has actually been done, in enough detail that someone who was not here can carry on"`
 	NextStep      string `json:"next_step" jsonschema:"the single next thing to do"`
-	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"what is blocking progress, if anything; required while the task is blocked"`
+	BlockedOn     string `json:"blocked_on,omitempty" jsonschema:"why the task is blocked. Only read while the task is in blocked: if you have hit a blocker, move the task to blocked with transition_task instead, because a blocker recorded here on an active task appears on no board"`
 }
 
 type appendWorklogIn struct {
 	Ref          string `json:"ref" jsonschema:"the task this attempt belongs to, like cairn-12"`
-	WhatWasTried string `json:"what_was_tried" jsonschema:"what you attempted"`
-	Outcome      string `json:"outcome,omitempty" jsonschema:"what happened as a result"`
+	WhatWasTried string `json:"what_was_tried" jsonschema:"what you actually did this stretch, in enough detail that someone about to repeat it would recognise it. The approach, not the intention"`
+	Outcome      string `json:"outcome,omitempty" jsonschema:"what happened, including failures worth not repeating"`
 }
```

```diff
--- a/internal/mcpserver/tools.go
+++ b/internal/mcpserver/tools.go
@@ func summariseDetail
 	if o.WorklogTotal > 0 {
 		if len(o.Worklog) < o.WorklogTotal {
-			line += fmt.Sprintf(". Showing the last %d of %d worklog entries.", len(o.Worklog), o.WorklogTotal)
+			line += fmt.Sprintf(". Showing the last %d of %d worklog entries; the abandoned "+
+				"approaches are usually among the older ones -- pass worklog_limit to read them.",
+				len(o.Worklog), o.WorklogTotal)
 		} else {
 			line += fmt.Sprintf(". %d worklog entries.", o.WorklogTotal)
 		}
 	}
 	if len(o.CanMoveTo) > 0 {
 		line += fmt.Sprintf(" You can move it to: %s.", strings.Join(o.CanMoveTo, ", "))
 	} else {
-		line += " There is nothing you can move it to from here."
+		line += " " + nothingToDo(o.Status)
 	}
 	return line
 }
+
+// nothingToDo explains an empty can_move_to. The three statuses that produce one
+// -- backlog, review, done -- are not the same dead end, and an agent that reads
+// only "nothing you can do" has no way to tell waiting from finished from
+// not-yours-yet.
+func nothingToDo(status string) string {
+	switch workflow.Status(status) {
+	case workflow.Backlog:
+		return "Nothing here is yours to move: only the human moves a task out of backlog, by queueing it."
+	case workflow.Review:
+		return "Nothing here is yours to move: it is with the human now, waiting to be marked done."
+	case workflow.Done:
+		return "This one is finished."
+	default:
+		return "There is nothing you can move it to from here."
+	}
+}
```

```diff
--- a/internal/workflow/workflow.go
+++ b/internal/workflow/workflow.go
@@ func (e *TransitionError) suffix
 	if len(e.Alternatives) == 0 {
-		return fmt.Sprintf("; from %s you cannot move this task", e.From)
+		return fmt.Sprintf("; from %s no move is yours to make -- the human moves it on from here", e.From)
 	}
```

True for all three cases: backlog waits to be queued, review waits to be marked
done or sent back, done waits to be reopened. The current clause tells an agent
it is stuck; this one tells it who is not.

# 2. Failure paths, walked

Every refusal an agent can reach, and what it actually says. Measured against
this Cairn over MCP.

| path | string | verdict |
|---|---|---|
| unknown project | `no project "cairn-tracker"; the projects here are: cairn, agac` | names the recovery. Keep. |
| unknown status | `unknown status "in_progress"; the statuses are backlog, queue, active, review, done, blocked` | keep; R6 stops it firing |
| unknown ref | `no task cairn-999` | see below |
| malformed ref | `"4" is not a task reference; they look like cairn-12` | names the shape. Keep. |
| empty state field | `state.where_i_left_off is empty; say what has actually been done` | keep, except when claiming — §3, T5 |
| empty worklog | `worklog.what_was_tried is empty; there is nothing to record` | keep |
| no such transition | `backlog -> active is not a transition in this workflow; from backlog you cannot move this task` | fails the bar — patched above |
| reserved for the human | `only the human marks work done; leave the task in review, and make sure state.next_step says what they should check` | the best string here |
| already there | `task is already active; from active you can move it to review, blocked` | keep |
| lost the race | `cairn-4 moved out of queue while this request was in flight; read it again` | keep |
| enum violation | `validating "arguments": validating root: validating /properties/to: enum: …` | R7 |
| no token | `cairn: this endpoint needs an agent token. Create one in the web interface under agents, and send it as Authorization: Bearer <token>.` | keep |

Two of these are worth a sentence each.

`no task cairn-999` is the one borderline case. It does not say what to do, and
the honest answer is that there is nothing to do — a ref either exists or it was
invented. The one recovery worth naming is the wrong-project case: `cairn-4` and
`agac-4` are both plausible typings of "task 4". I would leave it. Adding "the
projects here are: cairn, agac" to a not-found makes the common case noisier to
fix the rare one, and unlike `unknown project` the caller here has a ref that
parsed, so the project half was already accepted.

The last row is the only string on the surface that names the *web interface* as
the place to fix something. It is right to: an agent cannot mint its own token,
and the human is the one reading the logs.

# 3. Turn-burners, ranked by how often they bite

**T1. `list_tasks` returns no note at all.** tools.go:396 builds every row with
`toTaskOut(t, nil)`; `board` (tools.go:375) passes `row.State`. So `next_step`
and `updated_by` are structurally absent from `list_tasks` and present on
`board`, and the descriptions do not say so — `board`'s promises "each with the
note left on it" (tools.go:227), `list_tasks`'s says only "Every task in one
project" (tools.go:235).

*Failure:* an agent set up the way README.md:297-313 recommends — one repository,
one project — calls `list_tasks("myproject")` on a thirty-task project and gets
thirty rows of ref, title, status, timestamp. To find one sentence about any of
them it must `get_task`, thirty times, each answering with the body and ten
worklog entries. Or it calls `board` and filters client-side, which is strictly
better and is the wrong incentive to build into a pair of tools.

Same root as cairn-8: `store.Board` (store/task.go:267) already has the
`LEFT JOIN task_state` and no project filter; `ListTasksByProject`
(store/task.go:100) has the filter and no join. One `WHERE t.project_id = ?`
serves the MCP tool and the project page at once. **Bites: every project-scoped
listing, which is the documented normal way to use this product.**

**T2. The dead end at `can_move_to: []`.** Measured, on a task in backlog:

```
transition_task(ref: "cairn-4", to: "active", …)
  -> backlog -> active is not a transition in this workflow;
     from backlog you cannot move this task

get_task(ref: "cairn-4")
  -> … There is nothing you can move it to from here.
```

Both say what went wrong. Neither says what to do, and there is something to do
in all three cases (§1, R8).

*Failure:* the human says "have a look at cairn-4". The agent reads it, finds no
legal move and no reason, and does one of three things: stops with nothing
written; tries a move and spends a turn being told the move does not exist; or
files a near-duplicate task, because filing is the one write it knows it is
allowed. The third is the expensive one — it is a backlog entry that looks like
new information and is not.

This also covers the case an agent hits *after* doing everything right: it moves
work to review, re-reads the task, and is told there is nothing it can do —
which is true, and which reads identically to being stuck. **Bites: every time
an agent is pointed at unqueued work, which is how humans name tasks.**

**T3. `what_was_tried` is optional in the schema and required by the service for
the two commonest moves.** service/task.go:245-249 refuses `active -> review`
and `active -> blocked` without it. mcpserver_test.go:238-241 locks it optional
in the schema, correctly — making it unconditionally required would demand a
worklog entry for claiming work too.

*Failure:* the agent finishes, fills every field the schema marks required, and
is refused: `moving cairn-4 out of active requires worklog.what_was_tried;
record the attempt now, including what did not work`. A good message, a wasted
turn, on the single most frequent transition in the product. Fix is R5.

I considered and rejected expressing it in the schema with `allOf` +
`if`/`then`/`required`, which would not disturb the root `required` array the
test reads. The SDK validates schemas — the enum refusal in §2 is proof — so a
conditional `required` would replace that service message with
`validating /properties/what_was_tried: …`. That is the `to` enum's trade made a
second time, on a better string. **Bites: every completed task, until the agent
has been refused once.**

**T4. `Truncated` is true whenever the result exactly fills the limit.**
tools.go:373 and tools.go:394 both compute `Truncated: len(rows) == limit`.

*Failure:* `board(limit: 10)` on a Cairn with ten or more open tasks always
answers `10 tasks … (cut off at the limit; narrow by status or raise limit for
more)`. The agent raises the limit, gets the same rows, and has bought nothing.
Note this is nearly always wrong for the small explicit limit — which is the
case the flag was added for. Fix: ask the store for `limit+1`, return `limit`,
set the flag from the extra row. **Bites: whenever a count lands on a limit,
which for small limits is most of the time.**

**T5. Claiming queued work requires describing work not yet done.**
service/task.go:215-225: every agent transition needs state, and
`where_i_left_off` must be non-empty. For `queue -> active` — picking work up —
nothing has been done. The annotation says "what has actually been done"
(tools.go:46). The honest value is empty and empty is refused:
`state.where_i_left_off is empty; say what has actually been done`.

*Failure*, two shapes: a wasted turn if the agent answers honestly, and a junk
first note if it does not — "nothing yet", "starting now", "read the task" — in
the field the whole product is built around, written at the moment the product
first gets to see the agent.

There is a second half. `store.UpsertState` overwrites both columns, so if the
human left a note on the queued task saying where to start, claiming the task
destroys it. That is cairn-2's failure with the roles swapped, from the same
line of SQL, and I would fix them together.

I am not proposing to relax the rule — that state is mandatory on every move is
the product. I am proposing the annotation stop lying at the one moment the
field name does not fit, which is the diff in §1: *"When you are claiming queued
work, that is what you have read and understood; next_step is what you are about
to do first."* **Bites: every task an agent picks up. First write, every time.**

**T6. `create_task` needs a slug the agent has no way to know.** An agent that
has not called `list_projects` guesses. The refusal names the real ones and it
recovers in one turn, which is why this is last. Worth noting anyway: the
common self-hosted Cairn has exactly one project, where the guess is a coin flip
with a known answer. Making `project` optional and defaulting to the sole
project when there is exactly one removes the turn, and starts refusing — with
the list — the moment a second project exists. Small, and I would not do it
before T1. **Bites: the first `create_task` of a session, in a Cairn the agent
has not listed.**

# 4. What the connect-time instructions leave out

Three things an agent needs inside its first three calls and will not have.

**O1. That state and the worklog can be written without moving the task.**
mcpserver.go:29-58 describes both records, then describes only moves: "You
cannot move a task without writing state", "when you finish a piece of work,
move it to review", "when you are stuck, move it to blocked". `write_state` and
`append_worklog` are never named. The habit they exist for — checkpoint while
the work is long, because the note is the only thing that survives a crash — is
the single behaviour that makes this product work when an agent does not come
back, and the text an agent reads on connect does not ask for it. The tool
description says it (tools.go:277), and a tool description is read once the tool
is already under consideration. Two lines:

```
  Both records can be written without moving the task. On long work, write
  state as you go: if you stop unexpectedly, that note is what survives. Record
  an attempt with append_worklog as soon as you know how it went, rather than
  saving it all for the move.
```

**O2. That work starts in queue, and a backlog task is not yours yet.** The
instructions draw `backlog -> queue -> active` and say `backlog -> queue` is
refused, from which the conclusion follows if the agent joins two facts under
context pressure. The failure it prevents is T2, which is the most common dead
end on the surface. One clause:

```
Work you may pick up is in queue. A task in backlog is not yours to start; if
you think it should be worked on, say so and leave it where it is.
```

**O3. What to do with a task in active that nobody is working on.** The
instructions say do not leave a task in active. They say nothing about finding
one another agent left there — and there is no move for it: `active -> active`
is refused (`task is already active`), and taking work over is not a transition.
The one honest action available is `append_worklog` on a task the agent has not
claimed, which service/task.go:371-396 permits and nothing suggests. This is the
agent-side half of cairn-5: the human's board is getting a staleness mark; the
agent's board has neither the mark nor a sanctioned response.

```
If a task has been in active a long time with nothing written, the agent that
claimed it probably stopped. You cannot take it over -- that is the human's
call -- but you can append_worklog saying what you found, so the next reader is
not the first to notice.
```

Two things I checked and would **not** add. The lost-race conflict
(service/task.go:257-264) explains itself completely in its own message and
costs one re-read. The listing defaults — done excluded, fifty rows — are in the
tool descriptions and the summary admits truncation when it happens. Instruction
text is paid for on every connect by every agent; neither earns it.

# 5. Payload economy

The payloads are already lean, and I want to be plain about that before I
propose anything: I went looking for fields no agent will act on and found two
worth about four tokens between them (`taskDetailOut.Project`, tools.go:114,
restates the prefix of `Ref`; `projectOut.Name`, tools.go:97, is usually the
slug with a capital letter). Cutting them is not worth the diff. The `summarise`
mechanism (mcpserver.go:117-127) already removes the largest waste there was.

The real cost is the echo. `detailOf` (tools.go:476-482) answers **every** write
with a complete task read at `worklogQuery(0)`, which is ten worklog entries
(tools.go:484-489). For `write_state` — whose entire purpose is a cheap mid-run
checkpoint — that means each checkpoint pays for the body, the state it just
wrote, ten entries it already has, and a `can_move_to` that did not change. An
agent checkpointing five times through a long task has bought five `get_task`s
it did not ask for, and O1 above will make that more frequent, not less.

Keep the echo — `can_move_to` and the post-move status genuinely save a round
trip, which is why `detailOf` exists — and cut the history out of it. Writes
should answer at `WorklogLimit: 1`: for `append_worklog` and `transition_task`
that one entry is the one just written, which is a useful confirmation; for
`write_state` it is one line of context. `worklog_total` still reports the real
count, so nothing is hidden. `get_task` keeps its ten.

**The `worklog_limit` default, pressure-tested at 30 entries.** As asked.

`store.ListWorklog` (store/task.go:184-193) takes the newest ten and flips them
back into reading order, so a thirty-entry task returns entries 21-30 and the
summary says `Showing the last 10 of 30 worklog entries`.

The failure the worklog exists to prevent is proposing an approach that has
already been abandoned. On a task with thirty entries, the abandoned approach is
almost certainly **not** in the last ten — those are iterations of whatever is
being tried now. So the default trims at exactly the wrong end for the one thing
the record is for. The truncation is honest, but repairing it is not cheap: a
second `get_task` with a raised limit re-sends the body, the state, and the same
first ten entries, so you pay for the whole read twice to see the older twenty.

I do not propose raising the default. Ten is right for the common task, and a
thirty-entry task is by definition a task that has gone badly — taxing every
read to serve it is the wrong trade. R9 instead: make the truncation line an
instruction rather than a fact. It costs fourteen tokens on the reads that are
already truncated and nothing on the rest.

The change I am *not* confident enough to propose is in §7.

# 6. Can an agent end a session dishonestly?

Four ways. Only one of them is worth code.

**6.1 — The blocker written where nobody looks. Defend in code.**

This is the finding I would fix first, and it is not really about dishonesty: it
is an agent being careful and producing the exact artefact the product exists to
prevent.

- `WriteState` (service/task.go:348-351) refuses to *clear* `blocked_on` while a
  task is blocked. It does not refuse to *set* it while the task is not.
- `recordState` clears it only on a transition into active (workflow.go:192-193,
  service/task.go:320-322). No other path touches it.
- So: task is active, the agent hits a blocker, follows tools.go:57 — "what is
  blocking progress, **if anything**" — calls `write_state` with `blocked_on`
  filled, and does not move the task. Result: a task whose status says active
  and whose state says stuck.

Nobody sees it. `taskOut` carries no `blocked_on` at all (tools.go:68-76; this
is cairn-4). `TaskTable.vue:32` renders it only when
`row.task.status === 'blocked'`. `StatePanel.vue:15` — the task page, the
last place left — gates on the same condition. The field is write-only from the
human's side of the product: an agent can record a blocker in Cairn's own
designated blocker field and no human surface will ever render it.

Fix: `WriteState` should refuse a non-empty `blocked_on` on a task that is not
blocked, and name the move —

> cairn-4 is active, so state.blocked_on is not read there; if you are stuck,
> move it to blocked with transition_task, which is what puts the blocker on the
> board.

— plus R4 on the annotation. Silently dropping the value would be worse than
either: the agent would believe it had recorded the blocker, which is the
current situation with an extra step.

**6.2 — The note that satisfies the rule and says nothing. Prose.**

`where_i_left_off: "done"`, `next_step: "nothing"`, `what_was_tried: "worked on
it"`. Every check in the product is `strings.TrimSpace(x) != ""`
(service/task.go:220-224, :336-341, :375-377), so a one-word note passes all of
them.

Accept it, and fix it in prose. A minimum length buys "done working on it now" —
a longer lie in the same field, and worse, because it reads like a note. The
lever that actually moves this is the annotation: R1, R2, R3 and T5 all exist
because a field described in the words of its own name gets filled with the
words of its own name.

I did consider one cheap code defence and rejected it: refusing a state write
byte-identical to the state already stored, which would catch an agent
"checkpointing" by resubmitting. It is a real pattern, but a legitimate
transition immediately after a `write_state` carrying the same note would trip
it, and a refusal there teaches an agent to vary its prose rather than to
checkpoint honestly.

**6.3 — The task parked in active by an agent that stopped calling. Prose, plus
a readout the human owns.**

Not defensible at this layer, and I want to be precise about why rather than
restate cairn-5. Every obligation in Cairn is discharged at the moment of a
write. Stopping is the absence of one, and this server cannot see it: the
handler is mounted `Stateless: true` (mcpserver.go:79) and every tool
re-authenticates per request (mcpserver.go:90-107), so there is no session whose
end could be a hook. That is the right design and I am not proposing otherwise.

One thing is available that cairn-5 does not use. `token.last_used_at` is
already maintained, throttled to a minute (auth.go:28-31, store/token.go:63).
"This task has been in active for six hours and the actor that claimed it has
not presented a token in five" is a strictly better signal than the task's own
`updated_at`, because `updated_at` cannot distinguish an agent that is thinking
from an agent that is gone. It is data the product already writes. I would hand
that to whoever picks up cairn-5 rather than file it again — and I would not
build the agent-side version of it until the human-side threshold has been
watched for a while, for the reason cairn-5 gives.

**6.4 — The dead end an agent cannot report. Prose.**

The commonest honest ending has no documented form. An agent pointed at a
backlog task cannot move it, cannot queue it, and is told only that there is
nothing it can do (T2). It *can* `append_worklog` — service/task.go:371-396 has
no status check, so recording a finding on a task you have not claimed is fully
legal — and nothing on the surface suggests it. So the agent's choices are to
stop silently or to file a duplicate. O2, O3 and R8 together are the fix; none
of them is code.

**What I could not find:** a way to move a task while writing nothing. There
isn't one. `Transition` takes the note as an argument (service/task.go:195-286),
status, state and worklog go in one transaction, and the agent branch at :215
has no path around it. The central claim holds. Everything above is about the
quality of what gets written, not whether anything does.

# 7. What I am least sure about

**The worklog window.** I proposed a sentence (R9) where the honest fix might be
a shape: when the history is trimmed, return the oldest two entries alongside
the newest eight, because the abandoned approach is at the start and the current
state is at the end. I did not propose it, and I am not sure that was right. The
argument against is that it makes the array no longer a contiguous window, and
an agent reading a history with an unmarked hole in it may reason worse than one
reading a short honest slice. Marking the hole means a sentinel entry or a new
field, and at that point it is a design rather than a tweak. If R9 goes in and
agents still repeat old dead ends, this is the next thing to try — but somebody
should look at real thirty-entry tasks first, and this Cairn does not have one
yet.

**Whether R7 is worth its tokens.** It adds forty words to a field annotation
that the enum already constrains correctly. The argument for is that the enum
teaches an agent *what* it may do and never *why*, and "why" is what stops it
looking for a workaround. The argument against is that a model that reads the
enum has already been prevented from the mistake, and the words are paid for on
every `tools/list`. I lean toward including it because the same forty words also
answer "what happens to my task now" for an agent that has just moved something
to review — but I would not fight for it.

**T6, and whether defaulting `project` to the sole project is a good idea at
all.** It is a convenience that changes behaviour when a second project appears,
which is the kind of thing that works for six months and then confuses somebody
badly. I ranked it last for a reason and I would be happy to see it dropped.

**The one I am least sure of and rank highest anyway: §6.1.** I am confident the
field is unrendered — I read all three call sites. I am not confident about how
often an agent actually does it, because I could not measure it: this Cairn has
nine tasks and none has ever been blocked. The whole case rests on the claim
that "what is blocking progress, if anything" invites the write, and that is a
claim about how a model reads an annotation, made by a model reading an
annotation. It is the same evidence I have used for every rewrite in §1, and it
is weaker than the file:line evidence everywhere else in this report.

**A disagreement worth recording.** 01-product.md calls the human a reader and
the agent a writer, and treats that asymmetry as the unflattering thing. From
this surface it looks like the opposite problem: the agent is the one whose
records are shaped, checked and demanded, and every gap I found is a place where
the product knows exactly what it wants written and does not say so at the
moment of writing. That is not an asymmetry of obligation. It is an asymmetry of
instruction, and it is much cheaper to fix.
