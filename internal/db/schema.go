package db

import "database/sql"

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
CREATE TABLE IF NOT EXISTS jobs (
  id          TEXT PRIMARY KEY,
  command     TEXT NOT NULL,
  priority    INTEGER NOT NULL DEFAULT 0,
  status      TEXT NOT NULL DEFAULT 'pending',
  created_at  DATETIME NOT NULL DEFAULT (datetime('now')),
  started_at  DATETIME,
  finished_at DATETIME,
  exit_code   INTEGER,
  stdout      TEXT,
  stderr      TEXT
);
CREATE INDEX IF NOT EXISTS idx_status_priority ON jobs(status, priority DESC, created_at ASC);
`)
	return err
}
