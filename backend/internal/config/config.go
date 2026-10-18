// Package config loads isobuilder runtime configuration. Defaults come from
// environment variables; an optional YAML file (selected with -c) overrides
// them key by key. A deployment can therefore run from a single mounted file
// with no environment variables at all, while existing env-based setups keep
// working unchanged.
//
// Precedence: config file > environment variable > built-in default.
// Keys omitted from the file keep their environment/default value.
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
// -c; "" means environment variables only. A missing or unreadable file is an
// error, never a silent fallback — that failure mode is exactly what makes a
// "changed it but nothing happened" deployment hard to diagnose.
func Load(configPath string) (Config, error) {
	var fc fileConfig
	hasFile := false
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return Config{}, fmt.Errorf("read config file: %w", err)
		}
		if err := yaml.Unmarshal(data, &fc); err != nil {
			return Config{}, fmt.Errorf("parse config file %s: %w", configPath, err)
		}
		hasFile = true
	}

	dataDir := env("ISOBUILDER_DATA_DIR", "/opt/isobuilder/data")
	if hasFile && fc.DataDir != nil && strings.TrimSpace(*fc.DataDir) != "" {
		dataDir = strings.TrimSpace(*fc.DataDir)
	}
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
		DatabaseURL: strings.TrimSpace(os.Getenv("ISOBUILDER_DB_URL")),
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

	if hasFile {
		applyFile(fc, &cfg)
	}

	if cfg.MaxConcurrentBuilds < 1 {
		cfg.MaxConcurrentBuilds = 1
	}
	if cfg.MaxUploadBytes <= 0 {
		return Config{}, errors.New("max_upload_bytes must be positive")
	}
	if cfg.Storage.Enabled() {
		if cfg.Storage.AccessKey == "" || cfg.Storage.SecretKey == "" {
			return Config{}, errors.New("storage bucket set but access_key/secret_key missing")
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

// fileConfig mirrors Config as YAML. Pointer fields tell "key absent in the
// file" (nil, keep the environment/default) from "key explicitly set" — which
// matters for booleans and numbers, where the zero value is meaningful.
type fileConfig struct {
	Addr                *string `yaml:"addr"`
	DataDir             *string `yaml:"data_dir"`
	AuthToken           *string `yaml:"auth_token"`
	BuiltinDir          *string `yaml:"builtin_dir"`
	WebDir              *string `yaml:"web_dir"`
	DatabaseURL         *string `yaml:"database_url"`
	IsoDir              *string `yaml:"iso_dir"`
	UploadDir           *string `yaml:"upload_dir"`
	WorkDir             *string `yaml:"work_dir"`
	BuildDir            *string `yaml:"build_dir"`
	KickDir             *string `yaml:"kick_dir"`
	ArtifactDir         *string `yaml:"artifact_dir"`
	MaxUploadBytes      *int64  `yaml:"max_upload_bytes"`
	MaxConcurrentBuilds *int    `yaml:"max_concurrent_builds"`
	KeepWorkDir         *bool   `yaml:"keep_work_dir"`
	KeepLocalArtifact   *bool   `yaml:"keep_local_artifact"`
	BuildTimeoutMinutes *int    `yaml:"build_timeout_minutes"`
	Storage             *fileStorageConfig `yaml:"storage"`
}

// fileStorageConfig lets object storage be seeded from the file. The database
// settings row still wins at runtime (see settings.Load); this only replaces
// the environment as the initial default.
type fileStorageConfig struct {
	Endpoint          *string `yaml:"endpoint"`
	Region            *string `yaml:"region"`
	Bucket            *string `yaml:"bucket"`
	Prefix            *string `yaml:"prefix"`
	AccessKey         *string `yaml:"access_key"`
	SecretKey         *string `yaml:"secret_key"`
	PublicBase        *string `yaml:"public_base"`
	PresignTTLSeconds *int    `yaml:"presign_ttl_seconds"`
	PathStyle         *bool   `yaml:"path_style"`
	InsecureTLS       *bool   `yaml:"insecure_tls"`
}

// applyFile copies every key the file actually sets over the environment-
// derived config. Strings only override when non-empty, so an explicit empty
// value cannot accidentally blank out a required path.
func applyFile(fc fileConfig, cfg *Config) {
	applyStr(fc.Addr, &cfg.Addr)
	applyStr(fc.AuthToken, &cfg.AuthToken)
	applyStr(fc.BuiltinDir, &cfg.BuiltinDir)
	applyStr(fc.WebDir, &cfg.WebDir)
	applyStr(fc.DatabaseURL, &cfg.DatabaseURL)
	applyStr(fc.IsoDir, &cfg.IsoDir)
	applyStr(fc.UploadDir, &cfg.UploadDir)
	applyStr(fc.WorkDir, &cfg.WorkDir)
	applyStr(fc.BuildDir, &cfg.BuildDir)
	applyStr(fc.KickDir, &cfg.KickDir)
	applyStr(fc.ArtifactDir, &cfg.ArtifactDir)
	applyInt64(fc.MaxUploadBytes, &cfg.MaxUploadBytes)
	applyInt(fc.MaxConcurrentBuilds, &cfg.MaxConcurrentBuilds)
	applyBool(fc.KeepWorkDir, &cfg.KeepWorkDir)
	applyBool(fc.KeepLocalArtifact, &cfg.KeepLocalArtifact)
	if fc.BuildTimeoutMinutes != nil && *fc.BuildTimeoutMinutes > 0 {
		cfg.BuildTimeout = time.Duration(*fc.BuildTimeoutMinutes) * time.Minute
	}
	if fc.Storage != nil {
		applyStr(fc.Storage.Endpoint, &cfg.Storage.Endpoint)
		applyStr(fc.Storage.Region, &cfg.Storage.Region)
		applyStr(fc.Storage.Bucket, &cfg.Storage.Bucket)
		applyStr(fc.Storage.Prefix, &cfg.Storage.Prefix)
		applyStr(fc.Storage.AccessKey, &cfg.Storage.AccessKey)
		applyStr(fc.Storage.SecretKey, &cfg.Storage.SecretKey)
		applyStr(fc.Storage.PublicBase, &cfg.Storage.PublicBase)
		if fc.Storage.PresignTTLSeconds != nil && *fc.Storage.PresignTTLSeconds > 0 {
			cfg.Storage.PresignTTL = time.Duration(*fc.Storage.PresignTTLSeconds) * time.Second
		}
		applyBool(fc.Storage.PathStyle, &cfg.Storage.PathStyle)
		applyBool(fc.Storage.InsecureTLS, &cfg.Storage.InsecureTLS)
	}
}

func applyStr(src *string, dst *string) {
	if src == nil {
		return
	}
	if v := strings.TrimSpace(*src); v != "" {
		*dst = v
	}
}

func applyInt64(src *int64, dst *int64) {
	if src != nil && *src != 0 {
		*dst = *src
	}
}

func applyInt(src *int, dst *int) {
	if src != nil && *src != 0 {
		*dst = *src
	}
}

func applyBool(src *bool, dst *bool) {
	if src != nil {
		*dst = *src
	}
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
