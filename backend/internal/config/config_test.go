package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// writeConfig writes a YAML file and returns its path.
func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

// The file is the single source of truth for everything except the database
// credential, which may also come from the environment.
func TestLoadEverythingFromFile(t *testing.T) {
	dir := t.TempDir()
	root := filepath.Join(dir, "data")
	path := writeConfig(t, dir, "addr: \":9090\"\n"+
		"data_dir: "+root+"\n"+
		"auth_token: secret-token\n"+
		"max_concurrent_builds: 4\n"+
		"max_upload_bytes: 1024\n"+
		"keep_work_dir: true\n"+
		"keep_local_artifact: true\n"+
		"build_timeout_minutes: 30\n")

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
	if cfg.AuthToken != "secret-token" {
		t.Errorf("AuthToken = %q", cfg.AuthToken)
	}
	if cfg.MaxConcurrentBuilds != 4 {
		t.Errorf("MaxConcurrentBuilds = %d, want 4", cfg.MaxConcurrentBuilds)
	}
	if cfg.MaxUploadBytes != 1024 {
		t.Errorf("MaxUploadBytes = %d, want 1024", cfg.MaxUploadBytes)
	}
	if !cfg.KeepWorkDir || !cfg.KeepLocalArtifact {
		t.Errorf("keep flags = %v/%v, want true/true", cfg.KeepWorkDir, cfg.KeepLocalArtifact)
	}
	if cfg.BuildTimeout.Minutes() != 30 {
		t.Errorf("BuildTimeout = %v, want 30m", cfg.BuildTimeout)
	}
	// Every content directory derives from data_dir; overriding data_dir must
	// relocate all of them, including the SQLite file.
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
}

// Everything the file omits falls back to the documented defaults. data_dir is
// pointed at a temp dir because Load creates the tree, and the container
// default (/opt/isobuilder/data) is not writable on a developer machine.
func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "data_dir: "+filepath.Join(dir, "data")+"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want :8080", cfg.Addr)
	}
	if cfg.MaxConcurrentBuilds != 1 {
		t.Errorf("MaxConcurrentBuilds = %d, want 1", cfg.MaxConcurrentBuilds)
	}
	if cfg.MaxUploadBytes != 40<<30 {
		t.Errorf("MaxUploadBytes = %d, want 40GiB", cfg.MaxUploadBytes)
	}
	if cfg.BuildTimeout != 180*time.Minute {
		t.Errorf("BuildTimeout = %v, want 180m", cfg.BuildTimeout)
	}
	if cfg.KeepWorkDir || cfg.KeepLocalArtifact {
		t.Error("keep flags should default to false")
	}
	if cfg.AuthToken != "" {
		t.Error("auth_token should default empty (no auth)")
	}
	if cfg.Storage.Enabled() {
		t.Error("storage should default disabled")
	}
	// The container/bare-metal layout constants themselves.
	if defaultDataDir != "/opt/isobuilder/data" {
		t.Errorf("defaultDataDir = %q", defaultDataDir)
	}
	if defaultWebDir != "/opt/isobuilder/web" || defaultBuiltinDir != "/opt/isobuilder/kickstarts" {
		t.Errorf("web/builtin defaults = %q/%q", defaultWebDir, defaultBuiltinDir)
	}
}

// A named but missing file is a hard error: never silently fall back, that is
// the failure mode this flag exists to avoid.
func TestLoadMissingFileErrors(t *testing.T) {
	if _, err := Load(filepath.Join(t.TempDir(), "nope.yaml")); err == nil {
		t.Fatal("expected error for missing config file")
	}
}

// A file without a given key keeps the default for it.
func TestLoadFileWithoutKeyKeepsDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeConfig(t, dir, "data_dir: "+filepath.Join(dir, "d")+"\n")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Addr != ":8080" {
		t.Errorf("Addr = %q, want default", cfg.Addr)
	}
	if cfg.MaxConcurrentBuilds != 1 {
		t.Errorf("MaxConcurrentBuilds = %d, want default", cfg.MaxConcurrentBuilds)
	}
}

// The database credential may be injected through the environment so a
// deployment does not have to write the password to disk. A value in the file
// still wins, which keeps "one file describes the deployment" true when both
// are present.
func TestDatabaseURLEndEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DB_URL", "postgres://env@env-host/db")

	path := writeConfig(t, dir, "data_dir: "+filepath.Join(dir, "d")+"\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "postgres://env@env-host/db"; cfg.DatabaseURL != want {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

func TestDatabaseURLFromFileWins(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ISOBUILDER_DB_URL", "postgres://env@env-host/db")

	path := writeConfig(t, dir, "data_dir: "+filepath.Join(dir, "d")+"\n"+
		"database_url: mysql://file@file-host/db\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if want := "mysql://file@file-host/db"; cfg.DatabaseURL != want {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, want)
	}
}

// An endpoint without an explicit path_style implies a path-style store, which
// is what MinIO/Ceph/JD OSS need. The file can still say otherwise.
func TestStoragePathStyleDefault(t *testing.T) {
	dir := t.TempDir()
	base := "data_dir: " + filepath.Join(dir, "d") + "\n"

	path := writeConfig(t, dir, base+"storage:\n  endpoint: https://oss.example.com\n  bucket: b\n"+
		"  access_key: ak\n  secret_key: sk\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.Storage.PathStyle {
		t.Error("PathStyle = false, want true when endpoint is set")
	}

	path = writeConfig(t, dir, base+"storage:\n  endpoint: https://oss.example.com\n  bucket: b\n"+
		"  access_key: ak\n  secret_key: sk\n  path_style: false\n")
	cfg, err = Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Storage.PathStyle {
		t.Error("PathStyle = true, want false when the file says so")
	}
}
