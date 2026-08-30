# Cairn

An issue tracker that treats the coding agent as a first-class actor.

A cairn is a stack of stones left on a trail so whoever comes next knows the way.
Every task here carries a note for whoever picks it up, human or agent.

Existing trackers are designed human-first, with agent access bolted on afterwards
as a third-party add-on. Cairn inverts that: agent behaviour is in the schema and
the business rules, and the MCP server is in the same binary as the web interface,
calling the same service layer.

Self-hosted, single binary, SQLite. One human, N agents.

## The idea

Every task carries two records, and they do different jobs:

**state** — one note, overwritten in place, always current. `where_i_left_off`,
`next_step`, `blocked_on`. There is exactly one per task, enforced by the primary
key. It is the note on top of the cairn.

**worklog** — append-only. `what_was_tried`, `outcome`, who, when, and the status
change it accompanied. Never edited, never deleted. It is the trail behind you.

**An agent cannot leave a task without writing state.** Not a convention: the only
function that writes a task's status takes the note as an argument, and status,
state and worklog are written in one transaction or not at all.

## The workflow

```
backlog → queue → active → review → done
                    ↓
                 blocked
```

Two moves are the human's alone, and an agent is refused them:

- `backlog → queue` — only the human decides what gets worked on
- `review → done` — only the human decides something is finished

So an agent that finishes a piece of work moves it to `review` and says in
`next_step` what to check. Agents may file new tasks, but only into `backlog`.

These rules live in one place (`internal/workflow`), are enforced in one place
(`internal/service`), and both the web interface and the MCP server call that same
code. There is no second permission model to keep in sync.

## Running it

```bash
make build
./cairn
```

Open http://127.0.0.1:7777 and choose a username and password. That is the whole
setup: no registration, no email, no SMTP.

| flag | default | |
|---|---|---|
| `-db` | `cairn.db` | database file; created if absent |
| `-addr` | `127.0.0.1:7777` | use `:7777` to accept connections from other machines |
| `-secure-cookies` | `false` | set when serving over HTTPS |
| `-reset-password` | | set a new password and sign out every session |

Lost the password? There is no reset email and never will be. The only way back in
is access to the machine:

```bash
./cairn -reset-password
```

## Connecting an agent

In the web interface, go to **agents**, add one, and copy the token. It is shown
once and stored only as a hash; if you lose it, issue another and revoke the old.

Point the agent at `/mcp`, which speaks MCP over streamable HTTP:

```bash
claude mcp add --transport http cairn http://127.0.0.1:7777/mcp \
  --header "Authorization: Bearer cairn_..."
```

Or, for a client configured by file:

```json
{
  "mcpServers": {
    "cairn": {
      "type": "http",
      "url": "http://127.0.0.1:7777/mcp",
      "headers": { "Authorization": "Bearer cairn_..." }
    }
  }
}
```

Eight tools: `board`, `list_projects`, `list_tasks`, `get_task`, `create_task`,
`transition_task`, `write_state`, `append_worklog`. Every task read includes
`can_move_to` — the moves available to that agent from where the task is now — so
an agent never has to guess at a transition.

## Development

```bash
make build   # npm build, then go build with the frontend embedded
make test    # go test ./...
make dev     # prints the two commands for hot-reloading the frontend
```

`go build` on its own produces a working API with no web interface; the binary
says so on startup. The frontend is embedded from `web/dist` via `embed.FS`, so a
release is one file.

Layout:

```
internal/workflow/   the state machine. stdlib only, no database, no HTTP.
internal/model/      types that cross layer boundaries
internal/store/      SQL only, no rules
internal/service/    every rule; the only package that writes
internal/httpapi/    JSON API for the browser
internal/mcpserver/  MCP tools for agents
web/                 Vue 3 app, embedded into the binary
```

Two tests guard the architecture rather than behaviour: one walks the import graph
and fails if anything outside `internal/service` imports `internal/store`; the
other reflects over the service and fails if a method exists that has not declared
who may call it.

## License

Apache-2.0 — see [LICENSE](LICENSE).
