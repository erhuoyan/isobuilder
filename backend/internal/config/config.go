// Package config loads isobuilder runtime configuration from environment
// variables, optionally overridden by a small YAML file selected with -c that
// exists for exactly one chicken-and-egg key: database_url. Everything else
// the operator tunes (object storage, auth) already lives in the database
// settings the UI edits, or in the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	Addr      string
	DataDir   string
	AuthToken string

	BuiltinDir string // directory holding built-in kickstart templates
	WebDir     string // directory holding the built admin SPA

	IsoDir      string
	UploadDir   string
	WorkDir     string
	BuildDir    string
	KickDir     string
	ArtifactDir string
	// DatabaseURL selects PostgreSQL when non-empty; empty → embedded SQLite at DBPath.
	DatabaseURL string
	DBPath      string

	MaxUploadBytes      int64
	MaxConcurrentBuilds int
	KeepWorkDir         bool
	KeepLocalArtifact   bool
	BuildTimeout        time.Duration

	Storage StorageConfig
}

// StorageConfig describes the optional S3-compatible object storage target.
type StorageConfig struct {
	Endpoint    string
	Region      string
	Bucket      string
	Prefix      string
	AccessKey   string
	SecretKey   string
	PublicBase  string
	PresignTTL  time.Duration
	PathStyle   bool
	InsecureTLS bool
}

// Enabled reports whether object storage is configured.
func (s StorageConfig) Enabled() bool { return s.Bucket != "" }

// Load resolves runtime configuration. configPath is the YAML file named with
// -c; "" means environment variables only. The file may set a single key,
// database_url, which wins over ISOBUILDER_DB_URL. A missing or unreadable
// file is an error, never a silent fallback.
func Load(configPath string) (Config, error) {
	var fileDB string
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("read config file: %w", err)
		}
		var fc minimalConfig
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return Config{}, fmt.Errorf("parse config file %s: %w", configPath, err)
		}
		fileDB = strings.TrimSpace(fc.DatabaseURL)
	}

	dataDir := env("ISOBUILDER_DATA_DIR", "/data")
	if !filepath.IsAbs(dataDir) {
		abs, err := filepath.Abs(dataDir)
		if err != nil {
			return Config{}, fmt.Errorf("resolve data dir: %w", err)
		}
		dataDir = abs
	}

	cfg := Config{
		Addr:      env("ISOBUILDER_ADDR", ":8080"),
		DataDir:   dataDir,
		AuthToken: os.Getenv("ISOBUILDER_AUTH_TOKEN"),

		BuiltinDir: env("ISOBUILDER_BUILTIN_DIR", "/opt/isobuilder/kickstarts"),
		WebDir:     env("ISOBUILDER_WEB_DIR", "/opt/isobuilder/web"),

		IsoDir:      filepath.Join(dataDir, "isos"),
		UploadDir:   filepath.Join(dataDir, "uploads"),
		WorkDir:     filepath.Join(dataDir, "work"),
		BuildDir:    filepath.Join(dataDir, "builds"),
		KickDir:     filepath.Join(dataDir, "kickstarts"),
		ArtifactDir: filepath.Join(dataDir, "artifacts"),
		DatabaseURL: firstNonEmpty(fileDB, strings.TrimSpace(os.Getenv("ISOBUILDER_DB_URL"))),
		DBPath:      filepath.Join(dataDir, "isobuilder.db"),

		MaxUploadBytes:      envBytes("ISOBUILDER_MAX_UPLOAD_BYTES", 40<<30),
		MaxConcurrentBuilds: envInt("ISOBUILDER_MAX_CONCURRENT_BUILDS", 1),
		KeepWorkDir:         envBool("ISOBUILDER_KEEP_WORK_DIR", false),
		KeepLocalArtifact:   envBool("ISOBUILDER_KEEP_LOCAL_ARTIFACT", false),
		BuildTimeout:        time.Duration(envInt("ISOBUILDER_BUILD_TIMEOUT_MINUTES", 180)) * time.Minute,

		Storage: StorageConfig{
			Endpoint:    strings.TrimSpace(os.Getenv("ISOBUILDER_S3_ENDPOINT")),
			Region:      env("ISOBUILDER_S3_REGION", "us-east-1"),
			Bucket:      strings.TrimSpace(os.Getenv("ISOBUILDER_S3_BUCKET")),
			Prefix:      strings.Trim(strings.TrimSpace(os.Getenv("ISOBUILDER_S3_PREFIX")), "/"),
			AccessKey:   os.Getenv("ISOBUILDER_S3_ACCESS_KEY"),
			SecretKey:   os.Getenv("ISOBUILDER_S3_SECRET_KEY"),
			PublicBase:  strings.TrimRight(strings.TrimSpace(os.Getenv("ISOBUILDER_S3_PUBLIC_BASE")), "/"),
			PresignTTL:  time.Duration(envInt("ISOBUILDER_S3_PRESIGN_TTL_SECONDS", 86400)) * time.Second,
			PathStyle:   envBool("ISOBUILDER_S3_PATH_STYLE", strings.TrimSpace(os.Getenv("ISOBUILDER_S3_ENDPOINT")) != ""),
			InsecureTLS: envBool("ISOBUILDER_S3_INSECURE_TLS", false),
		},
	}

	if cfg.MaxConcurrentBuilds < 1 {
		cfg.MaxConcurrentBuilds = 1
	}
	if cfg.MaxUploadBytes <= 0 {
		return Config{}, errors.New("ISOBUILDER_MAX_UPLOAD_BYTES must be positive")
	}
	if cfg.Storage.Enabled() {
		if cfg.Storage.AccessKey == "" || cfg.Storage.SecretKey == "" {
			return Config{}, errors.New("ISOBUILDER_S3_BUCKET set but ISOBUILDER_S3_ACCESS_KEY/ISOBUILDER_S3_SECRET_KEY missing")
		}
		if cfg.Storage.PresignTTL <= 0 {
			cfg.Storage.PresignTTL = 24 * time.Hour
		}
	}

	for _, dir := range []string{cfg.IsoDir, cfg.UploadDir, cfg.WorkDir, cfg.BuildDir, cfg.KickDir, cfg.ArtifactDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Config{}, fmt.Errorf("create %s: %w", dir, err)
		}
	}
	return cfg, nil
}

// minimalConfig is the full YAML schema: exactly one key. It exists because
// the database address must be known before the database can be opened.
type minimalConfig struct {
	DatabaseURL string `yaml:"database_url"`
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func env(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envBytes(key string, def int64) int64 {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 {
		return def
	}
	return n
}

func envBool(key string, def bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "":
		return def
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return def
	}
}
