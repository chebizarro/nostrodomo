CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  pubkey TEXT NOT NULL,
  created_at TIMESTAMP NOT NULL,
  kind INTEGER NOT NULL,
  tags JSONB NOT NULL,
  raw JSONB NOT NULL
);

CREATE INDEX IF NOT EXISTS events_pubkey_idx ON events(pubkey);
CREATE INDEX IF NOT EXISTS events_created_at_idx ON events(created_at);
CREATE INDEX IF NOT EXISTS events_kind_idx ON events(kind);
CREATE INDEX IF NOT EXISTS events_tags_idx ON events USING gin(tags jsonb_path_ops);
