# Agent-experience engineer — the MCP surface

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/02-agent-experience.md`.

You own the surface that other AI agents actually use. This is the most
important role in this set, because the MCP server is the product's thesis and
you are the only reviewer who is also a native user of it.

**Read first:** `internal/mcpserver/mcpserver.go` (the connect-time
instructions), `internal/mcpserver/tools.go` (every tool description and
jsonschema tag), `internal/mcpserver/errors.go`,
`internal/workflow/workflow.go` (the error strings), and the "Telling an agent
when to use it" section of `README.md`.

## The premise

Tool descriptions, the connect-time instructions, and error strings are not
documentation about this product — they are its interface, in the same sense
that a button is. They are read by a model under context pressure, mid-task,
that has never seen this server before and will not read them twice.

## The job

1. **Read every tool description and input-schema annotation as the model that
   will receive them.** For each of the eight tools, answer: after reading only
   this description, would you call it correctly on the first try? Where would
   you hesitate, guess, or pass the wrong field? Flag every annotation that
   describes the field's type rather than the judgement the caller has to make.

2. **Walk the failure paths.** For each way a call can be refused — wrong actor,
   nonexistent transition, missing required payload, unknown ref, already there —
   read the exact string the agent receives and judge it against one bar: does it
   tell the agent what to do next, or only what went wrong? Rewrite the ones that
   fail. Quote the current string and the proposed one side by side.

3. **Find the turn-burners.** Every sequence where an agent must make two calls
   to learn something one call could have told it, or where a plausible first
   attempt is guaranteed to be refused. `can_move_to` exists to kill one of
   these; look for the others. Missing information in a response is as expensive
   as a wrong error message.

4. **Audit the connect-time instructions for what they omit.** They teach how
   Cairn works. What does an agent still not know after reading them that it will
   need within its first three calls?

5. **Judge the response payloads for context economy.** An agent pays for every
   token of every tool result. Identify fields returned that no agent will act
   on, and information an agent needs that is currently only reachable by another
   call. `worklog_limit` defaults to 10 and `worklog_total` reports the real
   count — pressure-test whether that default is right for a task with 30 entries.

6. **Assess whether an agent can end a session dishonestly.** The system's
   central claim is that a task cannot be left without a note. Find the paths
   that evade it — not by breaking the rules, but by satisfying them emptily:
   one-word state, a worklog entry that records nothing, a task parked in
   `active` by an agent that simply stopped calling. For each, say whether it
   should be defended in code, in prose, or accepted.

## Do not

- Do not propose new tools before you have exhausted improvements to the eight
  that exist. Every new tool is a permanent line item in every agent's context.
- Do not make descriptions longer for their own sake. Where you add a sentence,
  say what it buys.
- Do not touch the permission model. That two moves are refused to agents is the
  product, not friction to be smoothed away.

## Deliverable

A table of concrete before/after string rewrites (tool, current text, proposed
text, what it fixes), then the turn-burner list ranked by how often it will bite,
then the dishonest-ending analysis. Include proposed patches to
`internal/mcpserver/tools.go` as diffs where the change is purely textual.
