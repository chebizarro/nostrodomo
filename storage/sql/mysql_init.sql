CREATE TABLE IF NOT EXISTS events (
  id VARCHAR(64) PRIMARY KEY,
  pubkey VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL,
  kind INT NOT NULL,
  tags JSON NOT NULL,
  raw JSON NOT NULL
);

CREATE INDEX IF NOT EXISTS events_pubkey_idx ON events(pubkey);
CREATE INDEX IF NOT EXISTS events_created_at_idx ON events(created_at);
CREATE INDEX IF NOT EXISTS events_kind_idx ON events(kind);
CREATE INDEX IF NOT EXISTS events_tags_idx ON events((JSON_EXTRACT(tags, '$')));
