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

// Every runtime knob is settable from the file; the file wins over env.
func TestLoadFileOverridesAll(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", dir)
	t.Setenv("ISOBUILDER_ADDR", ":8080")
	t.Setenv("ISOBUILDER_MAX_CONCURRENT_BUILDS", "1")
	t.Setenv("ISOBUILDER_KEEP_WORK_DIR", "true")

	root := filepath.Join(dir, "alt")
	path := filepath.Join(dir, "config.yaml")
	body := "addr: \":9090\"\n" +
		"data_dir: " + root + "\n" +
		"auth_token: secret-token\n" +
		"max_concurrent_builds: 4\n" +
		"max_upload_bytes: 1024\n" +
		"keep_work_dir: false\n" +
		"keep_local_artifact: true\n" +
		"build_timeout_minutes: 30\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":9090" {
		t.Errorf("Addr = %q, want :9090", cfg.Addr)
	}
	if cfg.DataDir != root {
		t.Errorf("DataDir = %q, want %q", cfg.DataDir, root)
	}
	// Every content directory derives from data_dir; none is configurable
	// separately, so overriding data_dir must relocate all of them.
	for name, got := range map[string]string{
		"isos":       cfg.IsoDir,
		"uploads":    cfg.UploadDir,
		"work":       cfg.WorkDir,
		"builds":     cfg.BuildDir,
		"kickstarts": cfg.KickDir,
		"artifacts":  cfg.ArtifactDir,
	} {
		if want := filepath.Join(root, name); got != want {
			t.Errorf("%s dir = %q, want %q", name, got, want)
		}
	}
	if want := filepath.Join(root, "isobuilder.db"); cfg.DBPath != want {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, want)
	}
	if cfg.AuthToken != "secret-token" {
		t.Errorf("AuthToken = %q", cfg.AuthToken)
	}
	if cfg.MaxConcurrentBuilds != 4 {
		t.Errorf("MaxConcurrentBuilds = %d, want 4", cfg.MaxConcurrentBuilds)
	}
	if cfg.MaxUploadBytes != 1024 {
		t.Errorf("MaxUploadBytes = %d, want 1024", cfg.MaxUploadBytes)
	}
	// Explicit false must beat the environment's true.
	if cfg.KeepWorkDir {
		t.Error("KeepWorkDir = true, want false (file overrides env)")
	}
	if !cfg.KeepLocalArtifact {
		t.Error("KeepLocalArtifact = false, want true")
	}
	if cfg.BuildTimeout.Minutes() != 30 {
		t.Errorf("BuildTimeout = %v, want 30m", cfg.BuildTimeout)
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
