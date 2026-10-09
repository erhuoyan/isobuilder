// Package settings stores operator-editable runtime settings (currently the
// object storage target) in the database, so a bare-metal deployment can be
// configured from the web UI instead of environment variables.
package settings

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"isobuilder/internal/config"
	"isobuilder/internal/store"
)

// StorageKey is the settings row holding the object storage configuration.
const StorageKey = "storage"

// Storage is the JSON shape exchanged with the UI. AccessKey/SecretKey are
// write-only: they are never echoed back, only reported as configured or not.
type Storage struct {
	Endpoint    string `json:"endpoint"`
	Region      string `json:"region"`
	Bucket      string `json:"bucket"`
	Prefix      string `json:"prefix"`
	PathStyle   bool   `json:"path_style"`
	InsecureTLS bool   `json:"insecure_tls"`
	PublicBase  string `json:"public_base"`

	PresignTTLSeconds int `json:"presign_ttl_seconds"`

	AccessKey string `json:"access_key,omitempty"`
	SecretKey string `json:"secret_key,omitempty"`

	// Read-only fields filled in by the API.
	AccessKeySet bool   `json:"access_key_set"`
	SecretKeySet bool   `json:"secret_key_set"`
	Enabled      bool   `json:"enabled"`
	Source       string `json:"source"`
}

// Load resolves the effective storage settings: the database row wins, the
// environment provides the initial default.
func Load(st *store.Store, env config.StorageConfig) Storage {
	s := FromConfig(env)
	s.Source = "env"
	raw, err := st.GetSetting(StorageKey)
	if err != nil || strings.TrimSpace(raw) == "" {
		return s
	}
	var stored Storage
	if err := json.Unmarshal([]byte(raw), &stored); err != nil {
		return s
	}
	stored.Source = "db"
	return stored
}

// Save persists the storage settings.
func Save(st *store.Store, s Storage) error {
	payload, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return st.SetSetting(StorageKey, string(payload))
}

// FromConfig converts environment configuration into the settings shape.
func FromConfig(c config.StorageConfig) Storage {
	return Storage{
		Endpoint:          c.Endpoint,
		Region:            c.Region,
		Bucket:            c.Bucket,
		Prefix:            c.Prefix,
		PathStyle:         c.PathStyle,
		InsecureTLS:       c.InsecureTLS,
		PublicBase:        c.PublicBase,
		PresignTTLSeconds: int(c.PresignTTL.Seconds()),
		AccessKey:         c.AccessKey,
		SecretKey:         c.SecretKey,
	}
}

// ToConfig converts settings back into a usable storage configuration.
func (s Storage) ToConfig() config.StorageConfig {
	ttl := time.Duration(s.PresignTTLSeconds) * time.Second
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return config.StorageConfig{
		Endpoint:    strings.TrimSpace(s.Endpoint),
		Region:      strings.TrimSpace(s.Region),
		Bucket:      strings.TrimSpace(s.Bucket),
		Prefix:      strings.Trim(strings.TrimSpace(s.Prefix), "/"),
		AccessKey:   strings.TrimSpace(s.AccessKey),
		SecretKey:   strings.TrimSpace(s.SecretKey),
		PublicBase:  strings.TrimRight(strings.TrimSpace(s.PublicBase), "/"),
		PresignTTL:  ttl,
		PathStyle:   s.PathStyle,
		InsecureTLS: s.InsecureTLS,
	}
}

// Validate reports why the settings cannot be used, if so.
func (s Storage) Validate() error {
	c := s.ToConfig()
	if c.Bucket == "" {
		if c.AccessKey != "" || c.SecretKey != "" || c.Endpoint != "" {
			return errors.New("填了对象存储的其他字段，但 bucket 为空")
		}
		return nil // storage disabled
	}
	if c.Endpoint != "" && !strings.HasPrefix(c.Endpoint, "http://") && !strings.HasPrefix(c.Endpoint, "https://") {
		return errors.New("endpoint 必须以 http:// 或 https:// 开头，例如 https://oss.example.com")
	}
	if c.AccessKey == "" || c.SecretKey == "" {
		return errors.New("bucket 已填写，access_key 与 secret_key 必须同时提供")
	}
	if c.PresignTTL < time.Minute {
		return errors.New("预签名有效期至少 60 秒")
	}
	return nil
}

// Public returns the settings with secrets stripped, for API responses.
func (s Storage) Public() Storage {
	s.AccessKeySet = s.AccessKey != ""
	s.SecretKeySet = s.SecretKey != ""
	s.Enabled = strings.TrimSpace(s.Bucket) != ""
	s.AccessKey = ""
	s.SecretKey = ""
	return s
}
