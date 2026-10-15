package store

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// testPostgresDSN is read once; an unset value selects SQLite so the default
// `go test ./...` keeps running without any database dependency.
var testPostgresDSN = strings.TrimSpace(os.Getenv("ISOBUILDER_TEST_DB_URL"))

// OpenTest opens a store for a unit test. When ISOBUILDER_TEST_DB_URL is set
// the store is a PostgreSQL database whose schema is unique to this test run
// (created and dropped around the test); otherwise an embedded SQLite file.
// Either way two tests never see each other's rows.
func OpenTest(tb testing.TB, sqlitePath string) *Store {
	tb.Helper()
	if testPostgresDSN == "" {
		st, err := OpenSQLite(sqlitePath)
		if err != nil {
			tb.Fatalf("open sqlite: %v", err)
		}
		tb.Cleanup(func() { st.Close() })
		return st
	}
	schema := fmt.Sprintf("test_%s", sanitizeSchema(tb.Name()))
	st, err := OpenPostgres(testPostgresDSN, schema)
	if err != nil {
		tb.Fatalf("open postgres schema %s: %v", schema, err)
	}
	tb.Cleanup(func() {
		st.DropTestSchema(schema)
		st.Close()
	})
	return st
}

// TestPostgresEnabled reports whether tests run against PostgreSQL.
func TestPostgresEnabled() bool { return testPostgresDSN != "" }

// DropTestSchema removes a test schema and everything in it. Called from
// test cleanup so repeated runs against the same database stay clean.
func (s *Store) DropTestSchema(schema string) {
	if s.dialect != "postgres" {
		return
	}
	// A schema cannot be dropped while a live connection's search_path points
	// into it; return to a clean public path first.
	_, _ = s.db.Exec(`SET search_path TO public`)
	if _, err := s.db.Exec(fmt.Sprintf(`DROP SCHEMA IF EXISTS %s CASCADE`, quoteIdent(schema))); err != nil {
		_ = err // best-effort: test hygiene, never crashes cleanup
	}
}

func sanitizeSchema(name string) string {
	out := make([]byte, 0, len(name))
	for _, c := range name {
		switch {
		case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9', c == '_':
			out = append(out, byte(c))
		default:
			out = append(out, '_')
		}
	}
	return string(out)
}
