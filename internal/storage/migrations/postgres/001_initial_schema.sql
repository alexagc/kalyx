-- Initial Kalyx PostgreSQL Schema Migration (001)

CREATE TABLE IF NOT EXISTS schema_migrations (
    version INTEGER PRIMARY KEY,
    applied_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    domain TEXT NOT NULL,
    name TEXT NOT NULL,
    type TEXT NOT NULL UNIQUE,
    description TEXT,
    owner_team TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_events_domain ON events(domain);
CREATE INDEX IF NOT EXISTS idx_events_owner_team ON events(owner_team);

CREATE TABLE IF NOT EXISTS event_versions (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    semver TEXT NOT NULL,
    status TEXT NOT NULL,
    json_schema TEXT NOT NULL,
    target_table TEXT,
    partition_by JSONB,
    pii_fields JSONB,
    created_by TEXT NOT NULL,
    approved_by TEXT,
    approved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE(event_id, semver)
);

CREATE INDEX IF NOT EXISTS idx_event_versions_event_id ON event_versions(event_id);
CREATE INDEX IF NOT EXISTS idx_event_versions_status ON event_versions(status);

CREATE TABLE IF NOT EXISTS change_proposals (
    id TEXT PRIMARY KEY,
    event_id TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    target_semver TEXT NOT NULL,
    proposed_schema TEXT NOT NULL,
    proposed_by TEXT NOT NULL,
    status TEXT NOT NULL,
    review_notes TEXT,
    is_breaking BOOLEAN NOT NULL DEFAULT FALSE,
    breaking_reason TEXT,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_proposals_event_id ON change_proposals(event_id);
CREATE INDEX IF NOT EXISTS idx_proposals_status ON change_proposals(status);

CREATE TABLE IF NOT EXISTS pat_tokens (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    owner_email TEXT NOT NULL,
    team TEXT NOT NULL,
    expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    last_used_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_tokens_hash ON pat_tokens(token_hash);
CREATE INDEX IF NOT EXISTS idx_tokens_owner ON pat_tokens(owner_email);
