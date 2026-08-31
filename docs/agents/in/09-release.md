# Release and operations engineer

Read `docs/agents/shared-context.md` first — it is the other half of this brief.
Write your report to `docs/agents/out/09-release.md`.

This ships as a binary that strangers download and run on their own machines, and
as a scratch Docker image. Your subject is everything between "the code is right"
and "the person running v0.1.0 gets to v0.2.0 without losing their database."

**Read first:** `.github/workflows/release.yml`, `.github/workflows/ci.yml`,
`.github/workflows/pages.yml`, `Dockerfile`, `compose.yaml`, `Makefile`,
`internal/store/db.go` (migrations, PRAGMAs, connection setup), and
`cmd/cairn/main.go`.

## The job

1. **The upgrade path, in detail.** A user replaces the binary and restarts. Walk
   what happens: migrations apply forward, but what about a database written by a
   newer version and opened by an older one, an interrupted migration, or a
   migration that fails halfway? Is there a backup taken, and if not, should there
   be? Answer for a tool whose whole state is one SQLite file people forget to
   back up.

2. **Backup and restore.** There is no documented procedure. With WAL mode
   active, a naive `cp cairn.db` is not sufficient. Decide whether this belongs in
   the product (a flag, an endpoint) or in the README (the right `sqlite3`
   incantation and a cron line), then deliver whichever you chose.

3. **Operational visibility.** Startup logging, request logging, what the process
   says when it cannot open the database, when the frontend is not embedded, when
   the port is taken. Judge signal handling and whether shutdown is clean with
   respect to SQLite. Keep it minimal: this is a personal tool, not a fleet, and
   there is no telemetry and never will be.

4. **Reproducibility and provenance of releases.** Are the binaries built from a
   clean checkout, are the checksums produced the way a careful user would verify
   them, is the version stamped into the binary and reported to MCP clients on
   connect and visible in the interface? Judge whether build provenance
   attestation is worth the complexity here.

5. **The Docker story.** `FROM scratch`, unprivileged uid, `/data` volume. Check
   the image has what it needs for outbound TLS and correct timestamps if it ever
   needs them, check the volume permissions on first run with a fresh volume, and
   check `compose.yaml` matches what the README claims.

6. **The install path.** The README offers a tarball and a `docker run`. Judge the
   friction of the fifteen seconds between "I want to try this" and "it is
   running", and propose the smallest improvement — without adding a hosted
   install script that pipes curl into a shell.

## Do not

- Do not add telemetry, crash reporting, update checks, or any phone-home. This
  runs on machines with no outbound network by design.
- Do not propose Kubernetes manifests, Helm charts, or a systemd unit generator.
  One person, one box; a paragraph in the README about systemd is the ceiling.
- Do not add a package manager release (brew, apt, aur) as a recommendation
  without accounting for who maintains it every release.

## Deliverable

Ranked findings with the failure each one prevents, stated concretely ("a user on
v0.3 opening a database written by v0.4 gets X"). Then the backup and restore
procedure, written out and tested against a real WAL-mode database. Then the
patches to the workflows or Dockerfile for anything you found.
