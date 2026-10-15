// Package store persists ISO library, kickstart templates, projects, builds and
// artifacts. SQLite is the embedded default (a file under the data directory);
// PostgreSQL is selected by setting ISOBUILDER_DB_URL. The owner chooses the
// active database, the query layer stays dialect-blind.
package store

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"strings"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"

	"isobuilder/internal/model"
)

// Store wraps the database. The server is single-process by design; on SQLite
// a single connection serialises writers and keeps SQLITE_BUSY away, on
// PostgreSQL the server-side transaction queue does that job.
type Store struct {
	db      *sql.DB
	dialect string // "sqlite" | "postgres"
	schema  string // non-empty → PostgreSQL schema for test isolation
}

// ErrNotFound is returned when a row does not exist.
var ErrNotFound = errors.New("not found")

// OpenSQLite opens the embedded database at the given file path.
func OpenSQLite(path string) (*Store, error) {
	dsn := "file:" + path + "?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		return nil, err
	}
	s := &Store{db: db, dialect: "sqlite"}
	if err := s.migrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// OpenPostgres opens a PostgreSQL database. dsn may be a URL
// (postgres://user:pass@host:5432/db?sslmode=prefer) or a key=value DSN
// (host=... user=... dBName=...). When schema is non-empty every statement
// runs inside that schema, which is how tests isolate runs from each other.
func OpenPostgres(dsn, schema string) (*Store, error) {
	if !strings.HasPrefix(dsn, "postgres://") && !strings.HasPrefix(dsn, "postgresql://") {
		// libpq-style DSN: google/required DBNAME can be spelled dbName=.
		dsn = strings.Replace(dsn, "dbName=", "dbname=", 1)
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	s := &Store{db: db, dialect: "postgres", schema: schema}
	if schema != "" {
		// A schema isolates every test run. Unqualified table lookups resolve
		// through search_path, which pgx honours via the DSN runtime parameter
		// so every pooled connection lands in the schema without connector work.
		if _, err := db.Exec(fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s`, quoteIdent(schema))); err != nil {
			db.Close()
			return nil, fmt.Errorf("create schema %s: %w", schema, err)
		}
		db.Close()
		db, err = sql.Open("pgx", injectSearchPath(dsn, schema))
		if err != nil {
			return nil, err
		}
		if err := db.Ping(); err != nil {
			db.Close()
			return nil, fmt.Errorf("connect postgres (schema %s): %w", schema, err)
		}
		s.db = db
	}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

// injectSearchPath rewrites a postgres URL or key=value DSN to pin the schema
// in the search_path, so every pooled connection lands in the test schema
// without wrapping the connector.
func injectSearchPath(dsn, schema string) string {
	searchPath := quoteIdent(schema)
	if strings.HasPrefix(dsn, "postgres://") || strings.HasPrefix(dsn, "postgresql://") {
		sep := "?"
		if strings.Contains(dsn, "?") {
			sep = "&"
		}
		return dsn + sep + "search_path=" + url.QueryEscape(searchPath)
	}
	// libpq key=value DSN
	if strings.Contains(dsn, "options=") {
		return dsn // caller already set options
	}
	return dsn + " options='--search_path=" + searchPath + "'"
}


func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
func (s *Store) rebind(query string) string {
	if s.dialect != "postgres" {
		return query
	}
	var b strings.Builder
	b.Grow(len(query) + 16)
	n := 0
	for i := 0; i < len(query); i++ {
		if query[i] != '?' {
			b.WriteByte(query[i])
			continue
		}
		n++
		fmt.Fprintf(&b, "$%d", n)
	}
	return b.String()
}

func (s *Store) q(query string) string { return s.rebind(query) }

// query/exec/queryRow rebind placeholders for the active dialect, so every
// store method passes ?-shaped SQL and stays blind to the database.
func (s *Store) query(query string, args ...any) (*sql.Rows, error) {
	return s.db.Query(s.rebind(query), args...)
}

func (s *Store) queryRow(query string, args ...any) *sql.Row {
	return s.db.QueryRow(s.rebind(query), args...)
}

func (s *Store) exec(query string, args ...any) (sql.Result, error) {
	return s.db.Exec(s.rebind(query), args...)
}

// placeholders builds the (?, ?, …) list for INSERT … VALUES. It takes a
// column count so the SQLite and PostgreSQL shape stay identical.
func (s *Store) placeholders(n int) string {
	if s.dialect == "postgres" {
		var b strings.Builder
		for i := 1; i <= n; i++ {
			if i > 1 {
				b.WriteByte(',')
			}
			fmt.Fprintf(&b, "$%d", i)
		}
		return b.String()
	}
	return strings.Repeat(",?", n)[1:]
}

// --- schema --------------------------------------------------------------

const sqliteSchema = `
CREATE TABLE IF NOT EXISTS isos (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  filename   TEXT NOT NULL,
  arch       TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  sha256     TEXT NOT NULL DEFAULT '',
  md5        TEXT NOT NULL DEFAULT '',
  remark     TEXT NOT NULL DEFAULT '',
  status     TEXT NOT NULL,
  source     TEXT NOT NULL,
  path       TEXT NOT NULL,
  error      TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS kickstarts (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  arch       TEXT NOT NULL,
  builtin    INTEGER NOT NULL DEFAULT 0,
  remark     TEXT NOT NULL DEFAULT '',
  path       TEXT NOT NULL,
  size_bytes INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scripts (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  remark     TEXT NOT NULL DEFAULT '',
  content    TEXT NOT NULL DEFAULT '',
  size_bytes INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  arch          TEXT NOT NULL,
  iso_id        TEXT NOT NULL DEFAULT '',
  kickstart_id  TEXT NOT NULL DEFAULT '',
  script_id     TEXT NOT NULL DEFAULT '',
  iso_label     TEXT NOT NULL DEFAULT '',
  output_name   TEXT NOT NULL DEFAULT '',
  post_script   TEXT NOT NULL DEFAULT '',
  hosts_format  TEXT NOT NULL DEFAULT 'yaml',
  hosts_raw     TEXT NOT NULL DEFAULT '',
  hosts_json    TEXT NOT NULL DEFAULT '',
  hosts_count   INTEGER NOT NULL DEFAULT 0,
  hosts_summary TEXT NOT NULL DEFAULT '[]',
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS builds (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL,
  project_name TEXT NOT NULL DEFAULT '',
  status       TEXT NOT NULL,
  stage        TEXT NOT NULL DEFAULT '',
  stage_index  INTEGER NOT NULL DEFAULT 0,
  stage_total  INTEGER NOT NULL DEFAULT 5,
  progress     INTEGER NOT NULL DEFAULT 0,
  arch         TEXT NOT NULL DEFAULT '',
  iso_id       TEXT NOT NULL DEFAULT '',
  iso_name     TEXT NOT NULL DEFAULT '',
  iso_label    TEXT NOT NULL DEFAULT '',
  kickstart_id TEXT NOT NULL DEFAULT '',
  hosts_json   TEXT NOT NULL DEFAULT '',
  hosts_count  INTEGER NOT NULL DEFAULT 0,
  output_name  TEXT NOT NULL DEFAULT '',
  post_script    TEXT NOT NULL DEFAULT '',
  work_dir     TEXT NOT NULL DEFAULT '',
  error        TEXT NOT NULL DEFAULT '',
  artifact_id  TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  started_at   TEXT NOT NULL DEFAULT '',
  finished_at  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_builds_project ON builds(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS artifacts (
  id             TEXT PRIMARY KEY,
  build_id       TEXT NOT NULL,
  filename       TEXT NOT NULL,
  size_bytes     INTEGER NOT NULL DEFAULT 0,
  sha256         TEXT NOT NULL DEFAULT '',
  md5            TEXT NOT NULL DEFAULT '',
  storage        TEXT NOT NULL,
  storage_key    TEXT NOT NULL DEFAULT '',
  url            TEXT NOT NULL DEFAULT '',
  url_expires_at TEXT NOT NULL DEFAULT '',
  local_path     TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_artifacts_build ON artifacts(build_id);

CREATE TABLE IF NOT EXISTS settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
`

// PostgreSQL schema: identical columns, BIGINT for sizes, BOOLEAN for flags.
// TEXT timestamps are kept on purpose — the whole API speaks RFC3339 strings.
const postgresSchema = `
CREATE TABLE IF NOT EXISTS isos (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  filename   TEXT NOT NULL,
  arch       TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  sha256     TEXT NOT NULL DEFAULT '',
  md5        TEXT NOT NULL DEFAULT '',
  remark     TEXT NOT NULL DEFAULT '',
  status     TEXT NOT NULL,
  source     TEXT NOT NULL,
  path       TEXT NOT NULL,
  error      TEXT NOT NULL DEFAULT '',
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS kickstarts (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  arch       TEXT NOT NULL,
  builtin    BOOLEAN NOT NULL DEFAULT FALSE,
  remark     TEXT NOT NULL DEFAULT '',
  path       TEXT NOT NULL,
  size_bytes BIGINT NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS scripts (
  id         TEXT PRIMARY KEY,
  name       TEXT NOT NULL,
  remark     TEXT NOT NULL DEFAULT '',
  content    TEXT NOT NULL DEFAULT '',
  size_bytes BIGINT NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS projects (
  id            TEXT PRIMARY KEY,
  name          TEXT NOT NULL,
  description   TEXT NOT NULL DEFAULT '',
  arch          TEXT NOT NULL,
  iso_id        TEXT NOT NULL DEFAULT '',
  kickstart_id  TEXT NOT NULL DEFAULT '',
  script_id     TEXT NOT NULL DEFAULT '',
  iso_label     TEXT NOT NULL DEFAULT '',
  output_name   TEXT NOT NULL DEFAULT '',
  post_script   TEXT NOT NULL DEFAULT '',
  hosts_format  TEXT NOT NULL DEFAULT 'yaml',
  hosts_raw     TEXT NOT NULL DEFAULT '',
  hosts_json    TEXT NOT NULL DEFAULT '',
  hosts_count   INTEGER NOT NULL DEFAULT 0,
  hosts_summary TEXT NOT NULL DEFAULT '[]',
  created_at    TEXT NOT NULL,
  updated_at    TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS builds (
  id           TEXT PRIMARY KEY,
  project_id   TEXT NOT NULL,
  project_name TEXT NOT NULL DEFAULT '',
  status       TEXT NOT NULL,
  stage        TEXT NOT NULL DEFAULT '',
  stage_index  INTEGER NOT NULL DEFAULT 0,
  stage_total  INTEGER NOT NULL DEFAULT 5,
  progress     INTEGER NOT NULL DEFAULT 0,
  arch         TEXT NOT NULL DEFAULT '',
  iso_id       TEXT NOT NULL DEFAULT '',
  iso_name     TEXT NOT NULL DEFAULT '',
  iso_label    TEXT NOT NULL DEFAULT '',
  kickstart_id TEXT NOT NULL DEFAULT '',
  hosts_json   TEXT NOT NULL DEFAULT '',
  hosts_count  INTEGER NOT NULL DEFAULT 0,
  output_name  TEXT NOT NULL DEFAULT '',
  post_script  TEXT NOT NULL DEFAULT '',
  work_dir     TEXT NOT NULL DEFAULT '',
  error        TEXT NOT NULL DEFAULT '',
  artifact_id  TEXT NOT NULL DEFAULT '',
  created_at   TEXT NOT NULL,
  started_at   TEXT NOT NULL DEFAULT '',
  finished_at  TEXT NOT NULL DEFAULT ''
);
CREATE INDEX IF NOT EXISTS idx_builds_project ON builds(project_id, created_at DESC);

CREATE TABLE IF NOT EXISTS artifacts (
  id             TEXT PRIMARY KEY,
  build_id       TEXT NOT NULL,
  filename       TEXT NOT NULL,
  size_bytes     BIGINT NOT NULL DEFAULT 0,
  sha256         TEXT NOT NULL DEFAULT '',
  md5            TEXT NOT NULL DEFAULT '',
  storage        TEXT NOT NULL,
  storage_key    TEXT NOT NULL DEFAULT '',
  url            TEXT NOT NULL DEFAULT '',
  url_expires_at TEXT NOT NULL DEFAULT '',
  local_path     TEXT NOT NULL DEFAULT '',
  created_at     TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_artifacts_build ON artifacts(build_id);

CREATE TABLE IF NOT EXISTS settings (
  key        TEXT PRIMARY KEY,
  value      TEXT NOT NULL DEFAULT '',
  updated_at TEXT NOT NULL
);
`

// migrate creates the schema and, for existing databases, adds columns that
// were introduced after the first release. ALTER … ADD COLUMN is idempotent
// by error-swallowing: a duplicate-column error means the migration ran.
func (s *Store) migrate() error {
	schema := sqliteSchema
	if s.dialect == "postgres" {
		schema = postgresSchema
	}
	if _, err := s.db.Exec(schema); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}
	for _, stmt := range []string{
		"ALTER TABLE isos ADD COLUMN md5 TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE projects ADD COLUMN post_script TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE builds ADD COLUMN post_script TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE artifacts ADD COLUMN md5 TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE kickstarts ADD COLUMN remark TEXT NOT NULL DEFAULT ''",
		"ALTER TABLE projects ADD COLUMN script_id TEXT NOT NULL DEFAULT ''",
	} {
		if _, err := s.db.Exec(stmt); err != nil {
			continue // duplicate column → already migrated
		}
	}
	// Builds left queued or running by a previous process are gone with it;
	// mark them failed so nothing stays silently stuck.
	if _, err := s.db.Exec(s.q(`UPDATE builds SET status=?, error=?, finished_at=?
	                      WHERE status IN (?,?)`),
		model.BuildFailed, "服务重启，构建中断", Now(), model.BuildQueued, model.BuildRunning); err != nil {
		return err
	}
	return nil
}
