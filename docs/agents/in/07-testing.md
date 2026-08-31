# QA and test engineer

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/07-testing.md`.

The Go packages carry roughly 2,500 lines of tests including two architecture
tests; the Vue app has none; CI runs `go vet`, `go test`, a full build, and four
smoke assertions against a running binary.

**Read first:** every `*_test.go` file, then `.github/workflows/ci.yml`, then
`internal/workflow/workflow.go` and `internal/service/task.go` so you know what
the rules are before you judge whether they are covered.

## The job

1. **Map coverage against the rules rather than against the lines.** The rules
   are enumerable: ten transition edges with an actor list each, the entry
   statuses, the four `Requirement` fields, and who may call each service method.
   Build the matrix and find the cells no test occupies. Run
   `go test ./... -cover` and report per-package numbers, but the matrix is the
   finding, not the percentage.

2. **Find the tests that would not fail if the code were wrong.** Assertions on
   incidental values, tests that exercise a path without checking the outcome,
   table tests whose cases collapse to the same assertion. Name them.

3. **Identify the highest-value tests that do not exist.** My expectation, to
   confirm or refute: the transactional invariant under a mid-write failure (does
   a failed worklog insert roll back the status change?), concurrent transitions
   of the same task, the migration path from an older database file, and the full
   agent journey over MCP end to end — connect, board, `get_task`, transition,
   refused transition, read the error.

4. **Judge the CI smoke test.** It starts the binary, checks the frontend is
   served, checks the setup flag, and checks `/mcp` returns 401 unauthenticated.
   What is the fifth assertion worth adding? Keep it to one or two — this must
   stay fast.

5. **Decide the frontend testing question and commit to an answer:** a small
   Vitest setup, extending Go's `httptest` coverage of the same flows, or
   nothing. Whichever you pick, state the cost in dependencies and CI seconds and
   the class of bug it catches.

## Do not

- Do not chase a coverage percentage. Cairn is small enough that the rules can be
  enumerated, and a matrix beats a number.
- Do not introduce mocks for the database. The tests run against real SQLite and
  should continue to.
- Do not add Playwright, Cypress, or any browser-driving dependency without a
  specific bug class it catches that nothing cheaper does.

## Deliverable

The rules-versus-tests matrix as a table with the gaps marked. Then the ranked
list of tests to add, each with the bug it would catch stated as a concrete
scenario. Then the weak-test list. Write the three highest-value tests in full,
in the style of the existing table tests, and confirm they pass.
