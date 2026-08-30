# Cairn

An issue tracker that treats the coding agent as a first-class actor.

A cairn is a stack of stones left on a trail so whoever comes next knows the way.
Every task here carries a note for whoever picks it up, human or agent.

Existing trackers are designed human-first, with agent access bolted on afterwards as a
third-party add-on. Cairn bakes agent behaviour into the schema and the business rules:
every task carries a **state** (a single, always-current summary of where the work stands)
and a **worklog** (append-only, never edited). An agent cannot leave a task without
writing state.

Self-hosted, single binary, SQLite. One human, N agents.

## Status

Under construction. Schema and service layer are in place; HTTP/Vue and MCP are next.

## License

Apache-2.0 — see [LICENSE](LICENSE).
