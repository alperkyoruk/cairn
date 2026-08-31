# Reports

One file per role, named after the prompt that produced it:
`in/06-security.md` → `out/06-security.md`.

An agent writes here. A human reads here. Nothing in this directory is
authoritative — these are readings of the codebase, not decisions about it, and
two of them will contradict each other.

## Front matter

Every report opens with this block, filled in:

```markdown
---
role:    security engineer
date:    2026-08-31
commit:  8d35604
scope:   internal/service/auth.go, internal/httpapi/, internal/store/token.go,
         release.yml. Did not read the Vue app.
---
```

`commit` matters more than it looks: a finding against a commit from three weeks
ago may already be fixed, and without the sha nobody can tell.

## The three rules that hold for every role

- **Evidence, not opinion.** Every finding cites `file:line` and states a
  concrete failure scenario — inputs or sequence, and the wrong result they
  produce. "This could be clearer" is not a finding.
- **Rank, and be willing to be wrong about the ranking.** Ten findings at equal
  confidence tell the reader nothing about which to read first.
- **End with what you are least sure about.** Not a disclaimer. Usually where
  the real problem is.

## Rewriting a report

If a report already exists for your role, read it, then write a new one that
says what changed — what is now fixed, what you disagree with, what is new.
Do not silently overwrite; a report that has quietly lost its predecessor's
findings is worse than no report.

Do not edit another role's file. Read it for context and say where you disagree
in your own.

## Triage

Findings you accept become tasks. If Cairn's own MCP server is connected, an
agent may file them into project `cairn` directly — they land in `backlog`,
which is correct: the agent proposed the work, the human decides what gets done.
