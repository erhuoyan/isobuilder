package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"isobuilder/internal/settings"
	"isobuilder/internal/storage"
)

// handleGetSettings returns the operator-editable settings with secrets masked.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	cur, err := s.store.GetSetting(settings.StorageKey)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "读取系统配置失败: "+err.Error())
		return
	}
	eff := settings.Load(s.store, s.cfg.Storage)
	writeJSON(w, http.StatusOK, map[string]any{
		"storage":     eff.Public(),
		"storage_raw": strings.TrimSpace(cur) != "",
	})
}

// handleUpdateSettings writes the object storage settings and switches the
// active backend immediately, so no restart is needed.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Storage *struct {
			Endpoint          *string `json:"endpoint"`
			Region            *string `json:"region"`
			Bucket            *string `json:"bucket"`
			Prefix            *string `json:"prefix"`
			PathStyle         *bool   `json:"path_style"`
			InsecureTLS       *bool   `json:"insecure_tls"`
			PublicBase        *string `json:"public_base"`
			PresignTTLSeconds *int    `json:"presign_ttl_seconds"`
			// Empty string keeps the stored value, which is how the UI avoids
			// round-tripping secrets it never received.
			AccessKey *string `json:"access_key"`
			SecretKey *string `json:"secret_key"`
		} `json:"storage"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Storage == nil {
		fail(w, http.StatusBadRequest, "invalid_settings", "缺少 storage 配置")
		return
	}

	next := settings.Load(s.store, s.cfg.Storage)
	in := req.Storage
	if in.Endpoint != nil {
		next.Endpoint = strings.TrimSpace(*in.Endpoint)
	}
	if in.Region != nil {
		next.Region = strings.TrimSpace(*in.Region)
	}
	if in.Bucket != nil {
		next.Bucket = strings.TrimSpace(*in.Bucket)
	}
	if in.Prefix != nil {
		next.Prefix = strings.TrimSpace(*in.Prefix)
	}
	if in.PathStyle != nil {
		next.PathStyle = *in.PathStyle
	}
	if in.InsecureTLS != nil {
		next.InsecureTLS = *in.InsecureTLS
	}
	if in.PublicBase != nil {
		next.PublicBase = strings.TrimSpace(*in.PublicBase)
	}
	if in.PresignTTLSeconds != nil {
		next.PresignTTLSeconds = *in.PresignTTLSeconds
	}
	if in.AccessKey != nil && *in.AccessKey != "" {
		next.AccessKey = strings.TrimSpace(*in.AccessKey)
	}
	if in.SecretKey != nil && *in.SecretKey != "" {
		next.SecretKey = strings.TrimSpace(*in.SecretKey)
	}
	// An explicit bucket of "" together with cleared credentials turns object
	// storage off, which is a legitimate configuration.
	if next.Bucket == "" {
		next.AccessKey = ""
		next.SecretKey = ""
	}

	if err := next.Validate(); err != nil {
		fail(w, http.StatusBadRequest, "invalid_settings", err.Error())
		return
	}
	if err := settings.Save(s.store, next); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "保存系统配置失败: "+err.Error())
		return
	}
	if err := s.applyStorage(r.Context(), next); err != nil {
		fail(w, http.StatusBadRequest, "storage_unreachable", "配置已保存，但对象存储连接失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"storage": next.Public()})
}

// applyStorage swaps the active artifact backend, probing the bucket first so a
// wrong endpoint or credential is reported while the operator is still looking
// at the settings form.
func (s *Server) applyStorage(ctx context.Context, next settings.Storage) error {
	cfg := next.ToConfig()
	backend := storage.New(cfg)
	if cfg.Enabled() {
		client := storage.NewS3(cfg)
		probeCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := client.Ping(probeCtx); err != nil {
			return err
		}
	}
	if s.blob != nil {
		s.blob.Set(backend)
	}
	return nil
}
