-- This baseline expects a fresh database. Legacy unversioned installations must
-- export and migrate their data explicitly; silently changing them is unsafe.
CREATE TABLE users (
    id bigserial PRIMARY KEY,
    username varchar(40) NOT NULL CHECK (char_length(username) BETWEEN 3 AND 40),
    email varchar(254) NOT NULL UNIQUE CHECK (email = lower(email)),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE sessions (
    id varchar(32) PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_hash varchar(64) NOT NULL UNIQUE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sessions_user_id_idx ON sessions(user_id);
CREATE INDEX sessions_expires_at_idx ON sessions(expires_at);

CREATE TABLE notes (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title varchar(200) NOT NULL CHECK (char_length(title) > 0),
    content text NOT NULL CHECK (octet_length(content) <= 100000),
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (id, user_id)
);
CREATE INDEX notes_user_id_id_idx ON notes(user_id, id DESC);

CREATE TABLE alarms (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    note_id bigint NOT NULL,
    alarm_time timestamptz NOT NULL,
    status varchar(16) NOT NULL DEFAULT 'scheduled' CHECK (status IN ('scheduled', 'fired')),
    fired_at timestamptz,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    -- Ownership is also enforced in SQL, including concurrent note deletion.
    FOREIGN KEY (note_id, user_id) REFERENCES notes(id, user_id) ON DELETE CASCADE,
    CHECK ((status = 'scheduled' AND fired_at IS NULL) OR (status = 'fired' AND fired_at IS NOT NULL))
);
CREATE INDEX alarms_user_id_id_idx ON alarms(user_id, id DESC);
CREATE INDEX alarms_due_idx ON alarms(alarm_time) WHERE status = 'scheduled';
