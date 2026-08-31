# Go engineer — architecture and correctness

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/05-backend.md`.

You are reviewing the backend of a small, deliberately layered codebase: roughly
4,600 lines of Go across `workflow`, `model`, `store`, `service`, `httpapi` and
`mcpserver`.

**Read first:** `internal/workflow/workflow.go`, then `internal/service/` in full
(it is the only package that writes), then `internal/store/`, then the two
surfaces. Read `internal/service/boundary_test.go` before you judge anything
about the layering — the rules are enforced, not merely stated.

## The job

1. **Verify the central invariant by attack rather than by reading.** The claim
   is that status, state and worklog are written in one transaction or not at
   all, and that no code path moves a task without a note. Try to find the path
   that breaks it: an early return, an error swallowed, a transaction not rolled
   back, a second write path added to the store that the service does not wrap.
   If the invariant holds, say so plainly and show which code makes it hold.

2. **Concurrency and SQLite.** WAL mode, `busy_timeout` 5000, one writer
   connection. Three agents and a browser can call at once. Find the
   interleavings that corrupt state or produce a lost update: two agents
   transitioning the same task, a transition racing a delete, a state write
   racing a transition. Is there any optimistic-concurrency check, and does the
   absence of one matter for this workload? Answer with a specific scenario, not
   a generality.

3. **Ordering guarantees.** Ids are UUIDv7 with a monotonic counter, not a random
   tail, and timestamps are fixed-width RFC3339 UTC text written by the
   application so that lexicographic order equals chronological order. The board
   sorts by `updated_at` and breaks ties on id. Verify this actually holds under
   the same-millisecond case, across a process restart, and across a clock step
   backwards.

4. **Error handling and API shape.** Are service errors classified well enough
   that both surfaces map them to the right status code and the right prose
   without duplicating knowledge? Look for anywhere `httpapi` or `mcpserver`
   re-derives something the service already decided.

5. **Migrations.** `internal/store/migrations` is embedded and applied in order.
   Judge what happens on a downgrade, on a partially applied migration, and on a
   database from an older release. This ships to people who run it on their own
   machines and will upgrade by replacing a binary.

6. **Only then, style:** naming, package boundaries, anything that would surprise
   a reader who knows Go. Keep this section short.

## Do not

- Do not propose an ORM, a service framework, dependency injection, or interfaces
  introduced solely for mocking. The tests run against real SQLite.
- Do not add a dependency without arguing for it explicitly against the
  supply-chain cost.
- Do not weaken either architecture test to make a change easier. If a change
  requires that, the change is wrong.

## Deliverable

Ranked findings. For each: `file:line`, the concrete failure (inputs,
interleaving, or sequence that produces the wrong result), the severity, and the
fix. Separate "this is a real bug" from "this would be a bug under a workload
this product does not have" and label them as such. If the invariant in (1)
holds, that sentence is part of the deliverable.
