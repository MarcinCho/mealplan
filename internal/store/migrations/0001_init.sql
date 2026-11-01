CREATE TABLE household (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE app_user (
    id TEXT PRIMARY KEY,
    login TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    created_at BIGINT NOT NULL
);

CREATE TABLE membership (
    user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    PRIMARY KEY (user_id, household_id)
);

CREATE TABLE invite (
    id TEXT PRIMARY KEY,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    role TEXT NOT NULL CHECK (role IN ('admin', 'member')),
    created_by TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    expires_at BIGINT NOT NULL,
    used_at BIGINT
);

CREATE TABLE session (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES app_user(id) ON DELETE CASCADE,
    household_id TEXT NOT NULL REFERENCES household(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    csrf_token TEXT NOT NULL,
    created_at BIGINT NOT NULL,
    last_seen BIGINT NOT NULL,
    expires_at BIGINT NOT NULL
);

CREATE TABLE audit_event (
    id TEXT PRIMARY KEY,
    household_id TEXT,
    user_id TEXT,
    type TEXT NOT NULL,
    at BIGINT NOT NULL,
    details TEXT NOT NULL
);

CREATE INDEX idx_invite_household ON invite (household_id);

CREATE INDEX idx_session_user ON session (user_id);

CREATE INDEX idx_membership_household ON membership (household_id)
