# Agent prompts

Ten role prompts for improving Cairn, each written for a fresh agent session
with no other context.

They are deliberately narrow. An agent told to "make Cairn better" will propose
labels, priorities and a Kanban board within four minutes, because that is what
every other tracker has — and every one of those is on the closed out-of-scope
list. So each prompt fences one surface, hands over the constraint that makes
this project unusual, and names the bad answer its role tends to give.

```
docs/agents/
  shared-context.md   the other half of every prompt: thesis, constraints, house style
  in/                 the prompts
  out/                the reports they produce, one file per role
```

## Running one

```bash
claude "$(cat docs/agents/in/02-agent-experience.md)"
```

The prompt tells the agent to read `shared-context.md` itself, so this is enough
in a session that has the repository. Pasting into a session without filesystem
access means pasting `shared-context.md` first.

One role per session. The value is in the independence of the readings, and a
single agent asked to wear five hats produces five shallow lists.

## The roles

| | prompt | owns |
|---|---|---|
| 01 | [product](in/01-product.md) | what "better" means here, and defending the closed scope |
| 02 | [agent experience](in/02-agent-experience.md) | the MCP surface: tool descriptions, error strings, turn economy |
| 03 | [design](in/03-design.md) | the interface, against the brief it was designed from |
| 04 | [frontend](in/04-frontend.md) | Vue correctness, accessibility, the untested half |
| 05 | [backend](in/05-backend.md) | the transactional invariant, concurrency, ordering, migrations |
| 06 | [security](in/06-security.md) | credentials, CSRF, agent authority, supply chain |
| 07 | [testing](in/07-testing.md) | rules-versus-tests coverage, and what CI does not catch |
| 08 | [documentation](in/08-documentation.md) | the first fifteen minutes, and the four prose surfaces |
| 09 | [release](in/09-release.md) | upgrade path, backup, provenance, the Docker story |
| 10 | [positioning](in/10-positioning.md) | the one-sentence claim, and the landing page |

## Order

**01 and 02 first** — they set what "better" means and the others inherit it.
Then **03, 05, 06 in parallel**; design, backend and security are independent
readings and should not see each other's conclusions first. Then **04 and 07**,
which act on what 03 and 05 found. Then **08, 09, 10**, once the product has
stopped moving.

02 is the one to run if you only run one. The MCP server is the product's
thesis, and an agent reviewing it is a native user of it.

## Reading the output

See [out/README.md](out/README.md) for the report convention. The short version:
front matter with a commit sha, `file:line` on every finding, a ranked list, and
a closing section on what the agent is least sure about.
