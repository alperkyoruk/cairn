# Cairn

[![CI](https://github.com/alperkyoruk/cairn/actions/workflows/ci.yml/badge.svg)](https://github.com/alperkyoruk/cairn/actions/workflows/ci.yml)
[![Apache-2.0](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)

An issue tracker that treats the coding agent as a first-class actor.
[**alperkyoruk.github.io/cairn**](https://alperkyoruk.github.io/cairn/)

A cairn is a stack of stones left on a trail so whoever comes next knows the way.
Every task here carries a note for whoever picks it up, human or agent.

Self-hosted. One binary, one SQLite file, no dependencies to install. Apache-2.0.

---

## Why

Existing trackers are designed human-first and bolt agent access on afterwards as
a third-party add-on. The agent gets a thin API over a data model that never
expected it, and the interesting part — what the agent actually did, what it
tried, where it got stuck — ends up as prose in a comment box, if it is recorded
at all.

Cairn inverts that. Agent behaviour is in the schema and in the business rules,
and the MCP server is in the same binary as the web interface, calling the same
service layer. There is no separate agent API to drift out of sync, and no second
permission model to keep honest.

## The idea

Every task carries two records, and they do different jobs.

**state** — one note, overwritten in place, always current.

| field | |
|---|---|
| `where_i_left_off` | what has actually been done |
| `next_step` | the single next thing to do |
| `blocked_on` | what is needed in order to continue |

There is exactly one per task, enforced by the primary key. It is the note on top
of the cairn.

**worklog** — append-only. `what_was_tried`, `outcome`, who, when, and the status
change it accompanied. Never edited, never deleted. It is the trail behind you,
and it is where the dead ends live so the next agent does not repeat them.

**An agent cannot leave a task without writing state.** This is not a convention
or a lint rule. The only function in the codebase that writes a task's status
takes the note as an argument, and status, state and worklog are written in one
transaction or not at all. There is no code path that moves a task and forgets.

## The workflow

```
backlog → queue → active → review → done
                    ↓
                 blocked
```

`blocked` is not a stage in the pipeline. It is a side state that `active` falls
into and returns from.

Two moves are the human's alone, and an agent asking for them is refused:

- **`backlog → queue`** — only the human decides what gets worked on
- **`review → done`** — only the human decides something is finished

So an agent that finishes a piece of work moves it to `review` and says in
`next_step` what to check. A task sitting in `review` is always waiting on you,
which is why the board marks those rows and counts them.

Agents may file new tasks, but only into `backlog`. Noticing follow-up work and
writing it down is the behaviour you want; deciding it gets done stays with you.

The rules live in one place (`internal/workflow`, pure data and pure functions),
are enforced in one place (`internal/service`, the only package that writes), and
both the web interface and the MCP server call that same code.

## User model

One human. N agents. No teams, roles, invitations, or sharing — there is nobody
to share with.

On first launch you pick a username and password. There is no registration, no
email, and no SMTP. If you forget the password the only way back in is access to
the machine:

```bash
cairn -reset-password
```

Agents authenticate with API tokens, stored as a hash plus a short prefix so you
can tell one of an agent's tokens from another. A token is shown once.

---

## Running it

### Download a binary

Grab the archive for your platform from the [latest release](https://github.com/alperkyoruk/cairn/releases/latest),
unpack it, and run it. It is one self-contained file — the web interface is
compiled into it, there is no runtime to install, and nothing else to deploy.

```bash
tar -xzf cairn_v0.1.0_darwin_arm64.tar.gz
./cairn
```

Then open <http://127.0.0.1:7777> and choose a username and password.

Binaries are published for macOS and Linux on both `amd64` and `arm64`, plus
Windows on `amd64`. `checksums.txt` accompanies every release.

### With Docker

```bash
docker run -d --name cairn \
  -p 127.0.0.1:7777:7777 \
  -v cairn-data:/data \
  ghcr.io/alperkyoruk/cairn:latest
```

The image is built `FROM scratch` — no base image, no libc, no shell, nothing
with a CVE feed — because the binary has no dynamic links. It runs as an
unprivileged uid and keeps the database in `/data`.

There is also a `compose.yaml` in the repository, which publishes to
`127.0.0.1:8083` and expects a reverse proxy in front.

### From source

```bash
make build
./cairn
```

Needs Go 1.25+ and Node 24+. `make build` compiles the Vue app into `web/dist`
and then embeds it in the Go binary.

`go build` on its own produces a working API and MCP server with no web
interface; the binary says so on startup. `go install` has the same limitation,
because the frontend is built rather than committed — use a release binary or
`make build` if you want the web interface.

### Flags

| flag | default | |
|---|---|---|
| `-db` | `cairn.db` | database file; created if absent |
| `-addr` | `127.0.0.1:7777` | listen address; `:7777` to accept remote connections |
| `-secure-cookies` | `false` | mark the session cookie `Secure`; set this behind HTTPS |
| `-reset-password` | | set a new password and revoke every session |

### Behind a reverse proxy

Cairn does not terminate TLS. It is a single-user tool that expects to sit behind
something that does — and it must, because the password and the session cookie
cross the wire on every request. Set `-secure-cookies` when you do.

A Caddy site block is the whole configuration:

```caddyfile
cairn.example.com {
	encode zstd gzip
	header {
		Strict-Transport-Security "max-age=31536000; includeSubDomains"
		X-Content-Type-Options    "nosniff"
		Referrer-Policy           "strict-origin-when-cross-origin"
		-Server
	}
	reverse_proxy 127.0.0.1:8083
}
```

If you would rather not expose it at all, bind to loopback and reach it over an
SSH tunnel:

```bash
ssh -N -L 7777:127.0.0.1:7777 you@your-server
```

---

## Connecting an agent

In the web interface, go to **Agents**, register one, and copy the token — it is
shown once and stored only as a hash.

```bash
claude mcp add --scope user --transport http cairn http://127.0.0.1:7777/mcp \
  --header "Authorization: Bearer cairn_..."
```

Use `--scope user` so it is available in every project; that is the point. Do not
use `--scope project`, which writes the token into a file in your repository.

For a client configured by file:

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

### The tools

| tool | |
|---|---|
| `board` | every task across every project, most recently touched first |
| `list_projects` | the projects, and the prefixes task references are built from |
| `list_tasks` | one project's tasks |
| `get_task` | the ask, the note, the full worklog, and the legal moves from here |
| `create_task` | file a follow-up; lands in `backlog` |
| `transition_task` | move a task, leaving a note behind |
| `write_state` | overwrite the note without moving the task |
| `append_worklog` | record one attempt |

Every task read includes `can_move_to` — the statuses that actor may move it to
from where it is now, forward-most first — so an agent never has to guess at a
transition and burn a turn being refused.

Refusals explain themselves, because an agent reads an error the way a person
reads documentation:

```
only the human marks work done; leave the task in review,
and make sure state.next_step says what they should check
```

### Telling an agent when to use it

Connecting the server teaches an agent *how* Cairn works: the rules arrive as
server instructions on connect, and again in every tool description. What none
of that says is *when* to reach for it — an agent working in some other
repository will not think to check the board or leave a note before it stops
unless you ask it to.

That part belongs in the `CLAUDE.md` of each repository you track. Something
like this is enough:

```markdown
## Issue tracking

Work on this repo is tracked in Cairn, project `myproject`.

- Before starting a task, `get_task` it and read the worklog. Someone has
  probably already tried something; do not repeat their dead ends.
- Claim work by moving it `queue -> active` before you begin, not after.
- On anything long-running, `write_state` as you go. If you stop unexpectedly,
  that note is the only thing that survives.
- Before you stop, always leave the task somewhere honest:
  - finished  -> `review`, with `next_step` saying what I should check
  - stuck     -> `blocked`, with `blocked_on` saying exactly what you need
  - Never leave a task in `active` when you are not working on it.
- If you notice other work that needs doing, `create_task` it into backlog
  rather than silently widening the task you were given.
```

The last two are the ones worth keeping. A task parked in `active` by an agent
that stopped is a lie the board tells you every time you open it, and an agent
that quietly expands its own scope is the reason the backlog exists.

---

## Development

```bash
make test    # go test ./...
make build   # npm build, then go build with the frontend embedded
make dev     # prints the two commands for hot-reloading the frontend
```

For frontend work, run `go run ./cmd/cairn` and `cd web && npm run dev` in two
terminals and use <http://localhost:5173>, which proxies `/api` to the Go process
so cookies stay same-origin.

### Layout

```
internal/workflow/   the state machine. stdlib only, no database, no HTTP.
internal/model/      the types that cross layer boundaries
internal/store/      SQL only, no rules
internal/service/    every rule; the only package that writes
internal/httpapi/    JSON API for the browser
internal/mcpserver/  MCP tools for agents
web/                 Vue 3 app, embedded into the binary via embed.FS
docs/                the design brief and the interface spec it produced
```

Two tests guard the architecture rather than any behaviour:

- **`TestOnlyServiceReachesTheStore`** parses every Go file in the repository and
  fails if anything outside `internal/service` imports `internal/store`. This is
  what makes "one service layer" a fact rather than a habit — an MCP tool cannot
  grow its own SQL, and `main` cannot reach past the rules.
- **`TestEveryServiceMethodDecidesWhoMayCallIt`** reflects over the service and
  fails if a method exists that has not declared which operation it requires, or
  been explicitly listed as needing no caller. Adding a method without thinking
  about permissions breaks the build.

### Design notes

Some decisions that are easy to undo by accident:

- **Ids are UUIDv7 with a monotonic counter**, not a random tail. The board sorts
  by `updated_at` and breaks ties on id; two tasks touched in the same
  millisecond must not reorder between queries.
- **Timestamps are text**, RFC3339 UTC with fixed millisecond precision, written
  by the application and never by the database. The fixed width is what makes
  lexicographic order equal chronological order.
- **State is a panel and the worklog is a rail.** A panel reads as a record, a
  rail reads as a history. That opposite geometry is what stops the state block
  being taken for the most recent comment; do not give the worklog a card.
- **The interface never derives legal moves.** It renders whatever `can_move_to`
  says, so the state machine has exactly one implementation.

### Out of scope

This list is closed, not a backlog:

Gantt · calendar view · per-project saved views · due dates · reminders ·
recurring tasks · labels · priorities · estimates · sprints · milestones ·
attachments · comments · subtasks · dependencies · CalDAV · SMTP · notifications
· mobile or desktop apps · multi-user anything.

There is one user. There is nobody to notify, nobody to assign to, and nobody to
share with.

---

## License

Apache-2.0 — see [LICENSE](LICENSE). Section 5 makes inbound contribution terms
explicit, so there is no separate CLA.
