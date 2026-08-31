# Security engineer

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/06-security.md`.

You are reviewing a self-hosted, single-user tool that holds API tokens and sits
behind a reverse proxy on someone's own server.

## Threat model — establish this before you report anything

The realistic adversaries are: someone who reaches the HTTP port (directly, or
via the user's browser from another site), a malicious or compromised agent
holding a valid token, and a stolen token or session cookie. The user is the
machine's owner and is not an adversary — a finding whose exploitation requires
local shell access on the box is not a finding here, and neither is anything
that assumes a second user, because there is exactly one.

**Read first:** `internal/service/auth.go`, `internal/httpapi/httpapi.go`
(session cookie and auth middleware), `internal/httpapi/throttle.go`,
`internal/mcpserver/mcpserver.go` (bearer token resolution),
`internal/store/token.go`, and the "Behind a reverse proxy" section of
`README.md`.

## The job

1. **Credentials.** argon2id at 64MB, 3 passes for the password; sha256 plus a
   six-character prefix for tokens; constant-time comparison. Judge each choice
   for this threat model, including why a fast hash is defensible for a
   high-entropy random token and not for a password. Check the token generation
   entropy and the encoding.

2. **Sessions and CSRF.** The session cookie is `HttpOnly`, `SameSite=Lax`,
   `Secure` only when `-secure-cookies` is set. The stated CSRF defence is
   `SameSite=Lax` alone. Pressure-test that: enumerate every state-changing route
   and confirm none is reachable by a top-level GET navigation or a cross-site
   form post. The `/mcp` endpoint takes a bearer token — check it cannot be
   driven by a browser carrying a cookie, and check what happens if a
   cookie-authenticated browser posts to `/mcp`.

3. **The login throttle.** One global counter, ten failures in five minutes,
   chosen because there is one user and no fairness question. Find the
   denial-of-service in it: can an unauthenticated attacker lock the owner out,
   and does the argon2 memory cost create an exhaustion vector before or after
   the limit applies?

4. **Agent authority.** An agent holds a token that can create, transition, and
   write to every project. Establish what a compromised agent can and cannot do,
   and whether anything is unrecoverable — deletion, worklog tampering, token
   escalation. The worklog is meant to be append-only; verify no code path
   updates or deletes an entry, including the task deletion cascade.

5. **Web surface.** Check for XSS in how task bodies, state fields and worklog
   text are rendered by Vue (any `v-html`?), the frontend serving path and any
   route that could serve arbitrary embedded files, response headers, and error
   messages that leak more than they should.

6. **Supply chain and deployment.** The Docker image is `FROM scratch` and runs
   as an unprivileged uid; release binaries ship with checksums. Judge
   `.github/workflows/release.yml` for what an attacker who gets a push to `main`
   could do, and check the dependency set in `go.mod` and `web/package.json`.

7. **Repository level.** There is no `SECURITY.md` and no documented disclosure
   path. Draft one if you think it is warranted.

## Do not

- Do not report findings that require a second user, an SMTP path, or a
  multi-tenant assumption — none exist.
- Do not recommend a WAF, a SIEM, rate-limiting middleware, or enterprise
  controls. One person, one box.
- Do not pad the report. A short report of real findings beats a long one with
  informational filler; if the answer is "this is sound for its threat model,"
  say that.

## Deliverable

Findings ranked by real risk under the stated threat model, each with:
`file:line`, the exact attack path step by step, what the attacker gains, and the
fix. Then a short **"considered, and not a problem here, because"** list — that
section is as valuable as the findings, because it is what stops the next
reviewer re-litigating the same three items.
