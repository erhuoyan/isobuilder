package config

import (
	"os"
	"path/filepath"
	"testing"
)

// Load must take database_url from the -c file and ignore the environment
// when the file sets it; the file otherwise leaves environment values alone.
func TestLoadConfigFileDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", dir)
	t.Setenv("ISOBUILDER_DB_URL", "postgres://env@env-host/db")

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("database_url: postgres://file@file-host:5432/db\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "postgres://file@file-host:5432/db"; cfg.DatabaseURL != want {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

// No -c and no file: the environment value stands, SQLite otherwise.
func TestLoadNoFileUsesEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", dir)
	t.Setenv("ISOBUILDER_DB_URL", "postgres://env@env-host/db")

	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "postgres://env@env-host/db"; cfg.DatabaseURL != want {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

// A named but missing file is a hard error: never silently fall back to env
// (that is the failure mode this flag exists to avoid).
func TestLoadMissingFileErrors(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", dir)

	if _, err := Load(filepath.Join(dir, "nope.yaml")); err == nil {
		t.Fatal("expected error for missing config file")
	}
}

// A file without database_url leaves whatever the environment provides.
func TestLoadFileWithoutKeyKeepsEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", dir)
	t.Setenv("ISOBUILDER_DB_URL", "postgres://env@env-host/db")

	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte("# empty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "postgres://env@env-host/db"; cfg.DatabaseURL != want {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}
