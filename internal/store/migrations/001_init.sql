-- Cairn initial schema.
--
-- Rules about *who* may do *what* are not here. The only thing the database
-- knows about the workflow is which status strings are legal; internal/workflow
-- owns the rest, and internal/service is the only code that writes.

CREATE TABLE actor (
    id            TEXT PRIMARY KEY,
    actor_type    TEXT NOT NULL CHECK (actor_type IN ('human','agent')),
    name          TEXT NOT NULL UNIQUE,
    password_hash TEXT,
    created_at    TEXT NOT NULL,
    CHECK (actor_type = 'human' OR password_hash IS NULL)
);

-- One human user, by construction. N agents.
CREATE UNIQUE INDEX actor_one_human ON actor (actor_type) WHERE actor_type = 'human';

CREATE TABLE token (
    id           TEXT PRIMARY KEY,
    actor_id     TEXT NOT NULL REFERENCES actor(id) ON DELETE CASCADE,
    name         TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    created_at   TEXT NOT NULL,
    expires_at   TEXT,
    last_used_at TEXT,
    revoked_at   TEXT
);

CREATE INDEX token_actor ON token (actor_id);

CREATE TABLE project (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    name        TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    next_number INTEGER NOT NULL DEFAULT 1,
    created_at  TEXT NOT NULL,
    archived_at TEXT
);

-- No ON DELETE clause on project_id on purpose: deleting a project that still
-- holds tasks is refused rather than silently taking them with it.
CREATE TABLE task (
    id         TEXT PRIMARY KEY,
    project_id TEXT NOT NULL REFERENCES project(id),
    number     INTEGER NOT NULL,
    title      TEXT NOT NULL,
    body       TEXT NOT NULL DEFAULT '',
    status     TEXT NOT NULL DEFAULT 'backlog'
               CHECK (status IN ('backlog','queue','active','review','done','blocked')),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE UNIQUE INDEX task_ref    ON task (project_id, number);
CREATE INDEX        task_recent ON task (updated_at);
CREATE INDEX        task_status ON task (status);

-- The cairn itself: one row per task, overwritten in place. task_id is the
-- primary key, so "a single always-current summary" is a fact about the
-- database rather than a convention. No row means nobody has left a note yet.
CREATE TABLE task_state (
    task_id          TEXT PRIMARY KEY REFERENCES task(id) ON DELETE CASCADE,
    where_i_left_off TEXT NOT NULL DEFAULT '',
    next_step        TEXT NOT NULL DEFAULT '',
    blocked_on       TEXT NOT NULL DEFAULT '',
    updated_by       TEXT NOT NULL REFERENCES actor(id),
    updated_at       TEXT NOT NULL
);

-- Append-only for the life of the task. Nothing in the codebase updates or
-- deletes a row here; deleting the task removes the whole record at once.
CREATE TABLE worklog (
    id             TEXT PRIMARY KEY,
    task_id        TEXT NOT NULL REFERENCES task(id) ON DELETE CASCADE,
    actor_id       TEXT NOT NULL REFERENCES actor(id),
    created_at     TEXT NOT NULL,
    what_was_tried TEXT NOT NULL DEFAULT '',
    outcome        TEXT NOT NULL DEFAULT '',
    from_status    TEXT,
    to_status      TEXT
);

CREATE INDEX worklog_task ON worklog (task_id, created_at);
