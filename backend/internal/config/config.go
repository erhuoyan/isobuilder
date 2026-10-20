// Package config loads isobuilder runtime configuration from a YAML file
// (selected with -c). Built-in defaults cover a standard container layout, so a
// bare `isobuilder` with no file still starts.
//
// The only environment variable read is ISOBUILDER_DB_URL: database credentials
// are the one value a deployment usually wants to inject (systemd
// EnvironmentFile, Kubernetes Secret) rather than write into a file. Everything
// else has exactly one source — the config file — so there is never a question
// of which layer won.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Defaults for a standard container layout. Bare-metal installs override
// data_dir (see deploy/isobuilder.service); the rest usually stay as-is.
const (
	defaultAddr       = ":8080"
	defaultDataDir    = "/opt/isobuilder/data"
	defaultBuiltinDir = "/opt/isobuilder/kickstarts"
	defaultWebDir     = "/opt/isobuilder/web"
)

// Config is the fully resolved runtime configuration.
type Config struct {
	// Addr is the HTTP listen address in Go's host:port form, passed to
	// net.Listen. An empty host (":8080") binds every interface — which is
	// what container port mapping needs; "127.0.0.1:8080" would make the
	// service unreachable from outside the container.
	Addr      string
	DataDir   string // root for isos/work/builds/…; subdirs derive from it
	AuthToken string

	BuiltinDir string // directory holding built-in kickstart templates
	WebDir     string // directory holding the built admin SPA

	IsoDir      string
	UploadDir   string
	WorkDir     string
	BuildDir    string
	KickDir     string
	ArtifactDir string
	// DatabaseURL selects PostgreSQL or MySQL when non-empty; empty → embedded
	// SQLite at DBPath.
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
// -c; "" means built-in defaults only. A named but unreadable file is an error,
// never a silent fallback — "changed it but nothing happened" is exactly the
// failure mode that is hard to diagnose.
//
// ISOBUILDER_DB_URL, when set, overrides database_url from the file: it exists
// so a secret can be injected without putting it on disk.
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

	dataDir := defaultDataDir
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
		Addr:      defaultAddr,
		DataDir:   dataDir,
		AuthToken: "",

		BuiltinDir: defaultBuiltinDir,
		WebDir:     defaultWebDir,

		IsoDir:      filepath.Join(dataDir, "isos"),
		UploadDir:   filepath.Join(dataDir, "uploads"),
		WorkDir:     filepath.Join(dataDir, "work"),
		BuildDir:    filepath.Join(dataDir, "builds"),
		KickDir:     filepath.Join(dataDir, "kickstarts"),
		ArtifactDir: filepath.Join(dataDir, "artifacts"),
		DatabaseURL: strings.TrimSpace(os.Getenv("ISOBUILDER_DB_URL")),
		DBPath:      filepath.Join(dataDir, "isobuilder.db"),

		MaxUploadBytes:      40 << 30,
		MaxConcurrentBuilds: 1,
		KeepWorkDir:         false,
		KeepLocalArtifact:   false,
		BuildTimeout:        180 * time.Minute,

		Storage: StorageConfig{
			Region:     "us-east-1",
			PresignTTL: 24 * time.Hour,
		},
	}

	if hasFile {
		applyFile(fc, &cfg)
	}
	// An explicitly configured endpoint almost always means a private,
	// path-style store (MinIO, Ceph RGW, JD OSS); keep that convenience unless
	// the file says otherwise.
	if cfg.Storage.Endpoint != "" && (fc.Storage == nil || fc.Storage.PathStyle == nil) {
		cfg.Storage.PathStyle = true
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
// file" (nil, keep the default) from "key explicitly set" — which matters for
// booleans and numbers, where the zero value is meaningful.
//
// The per-directory paths (iso_dir, work_dir, …) are deliberately NOT here:
// every one of them derives from data_dir, and exposing them invited
// configurations that split isos and work across filesystems — which breaks
// the artifact move (cross-device fallback to copy, needing 2× space).
type fileConfig struct {
	Addr                *string            `yaml:"addr"`
	DataDir             *string            `yaml:"data_dir"`
	AuthToken           *string            `yaml:"auth_token"`
	BuiltinDir          *string            `yaml:"builtin_dir"`
	WebDir              *string            `yaml:"web_dir"`
	DatabaseURL         *string            `yaml:"database_url"`
	MaxUploadBytes      *int64             `yaml:"max_upload_bytes"`
	MaxConcurrentBuilds *int               `yaml:"max_concurrent_builds"`
	KeepWorkDir         *bool              `yaml:"keep_work_dir"`
	KeepLocalArtifact   *bool              `yaml:"keep_local_artifact"`
	BuildTimeoutMinutes *int               `yaml:"build_timeout_minutes"`
	Storage             *fileStorageConfig `yaml:"storage"`
}

// fileStorageConfig seeds the object storage target. The database settings row
// still wins at runtime (see settings.Load); this only provides the initial
// default, replacing what used to come from environment variables.
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

// applyFile copies every key the file actually sets over the defaults.
// Strings only override when non-empty, so an explicit empty value cannot
// accidentally blank out a required path.
func applyFile(fc fileConfig, cfg *Config) {
	applyStr(fc.Addr, &cfg.Addr)
	applyStr(fc.AuthToken, &cfg.AuthToken)
	applyStr(fc.BuiltinDir, &cfg.BuiltinDir)
	applyStr(fc.WebDir, &cfg.WebDir)
	applyStr(fc.DatabaseURL, &cfg.DatabaseURL)
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
