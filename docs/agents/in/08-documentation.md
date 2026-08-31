# Technical writer — documentation and first-run experience

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/08-documentation.md`.

Cairn's documentation is unusually good and unusually long, and both facts are
problems worth examining.

**Read first:** `README.md` in full (369 lines), the connect-time instructions in
`internal/mcpserver/mcpserver.go`, the tool descriptions in
`internal/mcpserver/tools.go`, the error strings in
`internal/workflow/workflow.go`, the setup and login screens in
`web/src/views/`, and `site/index.html`.

## The premise

There are four documentation surfaces here and they are not equally good: the
README, the marketing site, the in-product prose, and the prose an agent reads
over MCP. The last two are the ones people actually meet.

## The job

1. **Walk the first fifteen minutes** as someone who found this on GitHub ten
   minutes ago: read the README's opening, download a binary, run it, pick a
   username, land on an empty board, and try to connect an agent. Write down
   every moment of hesitation with the exact line that caused it. The empty board
   is a critical juncture — a new user has no projects, no tasks, and no agent,
   and nothing has told them what to do first.

2. **Judge the README's structure against how it is read.** It currently runs:
   why, the idea, the workflow, the user model, running it, connecting an agent,
   development, design notes, out of scope. Someone deciding whether to try this
   and someone trying to install it need different first screens. Propose the
   restructure if you believe in it, or defend the current order — but decide.

3. **Audit the `CLAUDE.md` snippet the README suggests users paste into their own
   repositories.** That snippet is the single highest-leverage paragraph in the
   product, because it is what makes an agent reach for Cairn at the right moment
   in someone else's codebase. Rewrite it to be as short as it can be while still
   producing the behaviour, and say which lines earn their place.

4. **Check every command, flag, path and URL** in the README against the code.
   Report anything that has drifted.

5. **Name what is missing at the repository level** and draft what is warranted:
   no `CONTRIBUTING.md`, no `CHANGELOG.md`, no `SECURITY.md`, no issue templates.
   Be ruthless — for a single-maintainer Apache-2.0 project, some of these are
   ceremony, and saying so is a valid finding. Recommend at most two, and draft
   them.

6. **Read the in-product prose as copy:** empty states, the setup screen's
   warning that `-reset-password` is the only recovery path, the
   token-shown-once notice, the delete confirmation that must say the worklog
   goes with it, and every error the browser can show. Rewrite the ones that are
   merely accurate into ones that are useful.

## Do not

- Do not add marketing register, feature-benefit tables, emoji headers, or a
  "Features" section. The README's voice is plain and slightly severe; keep it.
- Do not document anything on the closed out-of-scope list, even as "not yet
  supported." That list is closed, and the README says so on purpose.
- Do not propose a documentation site. The README plus the landing page is the
  whole surface by design.

## Deliverable

The fifteen-minute walkthrough with hesitation points quoted line by line, the
drift report as a table, the rewritten `CLAUDE.md` snippet, the in-product copy
rewrites as before/after pairs, and at most two new repository files drafted in
full.
