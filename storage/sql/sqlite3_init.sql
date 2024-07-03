CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    pubkey TEXT,
    created_at INTEGER,
    kind INTEGER,
    raw TEXT
);

CREATE TABLE IF NOT EXISTS tags (
    event_id TEXT,
    name TEXT,
    value TEXT,
    other_parameters TEXT,
    FOREIGN KEY(event_id) REFERENCES events(id)
);

CREATE INDEX IF NOT EXISTS idx_tags_name ON tags(name);
CREATE INDEX IF NOT EXISTS idx_tags_value ON tags(value);
CREATE INDEX IF NOT EXISTS idx_tags_event_id ON tags(event_id);
