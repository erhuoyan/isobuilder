// Package httpapi exposes the REST API described in docs/API.md and serves the
// admin SPA from the same origin.
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"isobuilder/internal/builder"
	"isobuilder/internal/buildlog"
	"isobuilder/internal/config"
	"isobuilder/internal/diskutil"
	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// Version is the reported API version.
const Version = "0.1.10"

// BuildManager is the slice of builder.Manager the HTTP layer needs.
type BuildManager interface {
	Enqueue(b model.Build) error
	Cancel(buildID string) bool
	LogFile(buildID string) (string, int64, bool)
	Subscribe(buildID string) (<-chan buildlog.Line, func(), bool)
	Storage() storage.Backend
}

// Server wires the HTTP layer to the store and the build manager.
type Server struct {
	cfg     config.Config
	store   *store.Store
	manager BuildManager
	blob    *storage.Switcher
	uploads *uploadRegistry
	mux     *http.ServeMux
	log     *slog.Logger
}

// New builds the API server and registers every route. blob is the same
// swappable backend the build manager publishes through, so a settings change
// takes effect everywhere at once.
func New(cfg config.Config, st *store.Store, mgr BuildManager, blob *storage.Switcher, logger *slog.Logger) *Server {
	s := &Server{
		cfg:     cfg,
		store:   st,
		manager: mgr,
		blob:    blob,
		uploads: newUploadRegistry(cfg.UploadDir),
		mux:     http.NewServeMux(),
		log:     logger,
	}
	s.routes()
	return s
}

// Handler returns the root handler, including auth and static file serving.
func (s *Server) Handler() http.Handler {
	return s.withAuth(s.withRecovery(s.mux))
}

func (s *Server) routes() {
	m := s.mux

	m.HandleFunc("GET /api/health", s.handleHealth)
	m.HandleFunc("GET /api/config", s.handleConfig)

	// Operator-editable settings (object storage target)
	m.HandleFunc("GET /api/settings", s.handleGetSettings)
	m.HandleFunc("PUT /api/settings", s.handleUpdateSettings)

	// ISO library
	m.HandleFunc("GET /api/isos", s.handleListISOs)
	m.HandleFunc("POST /api/isos/uploads", s.handleStartUpload)
	m.HandleFunc("PUT /api/isos/uploads/{upload_id}", s.handleAppendUpload)
	m.HandleFunc("POST /api/isos/uploads/{upload_id}/finish", s.handleFinishUpload)
	m.HandleFunc("DELETE /api/isos/uploads/{upload_id}", s.handleCancelUpload)
	m.HandleFunc("POST /api/isos/register", s.handleRegisterISO)
	m.HandleFunc("DELETE /api/isos/{id}", s.handleDeleteISO)

	// Kickstart templates
	m.HandleFunc("GET /api/kickstarts", s.handleListKickstarts)
	m.HandleFunc("POST /api/kickstarts", s.handleCreateKickstart)
	m.HandleFunc("GET /api/kickstarts/{id}/content", s.handleKickstartContent)
	m.HandleFunc("PUT /api/kickstarts/{id}", s.handleUpdateKickstart)
	m.HandleFunc("DELETE /api/kickstarts/{id}", s.handleDeleteKickstart)

	// Script library（装机后脚本库）
	m.HandleFunc("GET /api/scripts", s.handleListScripts)
	m.HandleFunc("POST /api/scripts", s.handleCreateScript)
	m.HandleFunc("PUT /api/scripts/{id}", s.handleUpdateScript)
	m.HandleFunc("DELETE /api/scripts/{id}", s.handleDeleteScript)

	// Projects
	m.HandleFunc("GET /api/projects", s.handleListProjects)
	m.HandleFunc("POST /api/projects", s.handleCreateProject)
	m.HandleFunc("POST /api/projects/import", s.handleImportProject)
	m.HandleFunc("GET /api/projects/{id}", s.handleGetProject)
	m.HandleFunc("PUT /api/projects/{id}", s.handleUpdateProject)
	m.HandleFunc("DELETE /api/projects/{id}", s.handleDeleteProject)
	m.HandleFunc("POST /api/validate/hosts", s.handleValidateHosts)
	m.HandleFunc("POST /api/projects/{id}/builds", s.handleCreateBuild)

	// Builds
	m.HandleFunc("GET /api/builds", s.handleListBuilds)
	m.HandleFunc("GET /api/builds/{id}", s.handleGetBuild)
	m.HandleFunc("GET /api/builds/{id}/log", s.handleBuildLog)
	m.HandleFunc("POST /api/builds/{id}/cancel", s.handleCancelBuild)
	m.HandleFunc("POST /api/builds/{id}/retry", s.handleRetryBuild)

	// Artifacts
	m.HandleFunc("GET /api/artifacts/{id}", s.handleGetArtifact)
	m.HandleFunc("GET /api/artifacts/{id}/download", s.handleDownloadArtifact)

	// SPA
	m.HandleFunc("/", s.handleStatic)
}

// --- middleware ---------------------------------------------------------

func (s *Server) withAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.cfg.AuthToken == "" || !strings.HasPrefix(r.URL.Path, "/api/") {
			next.ServeHTTP(w, r)
			return
		}
		switch r.URL.Path {
		case "/api/health", "/api/config":
			next.ServeHTTP(w, r)
			return
		}
		token := ""
		if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
			token = strings.TrimPrefix(h, "Bearer ")
		}
		if token == "" {
			token = r.Header.Get("X-Auth-Token")
		}
		if token != s.cfg.AuthToken {
			// Authentication failure must stay a transport-level 401: flattening it
			// to HTTP 200 would hide it from the frontend's auth machinery, which
			// triggers logout/redirect from the status code, not the envelope.
			writeJSONBody(w, http.StatusUnauthorized, envelope{
				Code:    "401",
				Message: "访问令牌无效或缺失",
				Data:    map[string]any{},
			})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) withRecovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("panic in handler", "path", r.URL.Path, "recover", rec)
				fail(w, http.StatusInternalServerError, "internal", "服务内部错误，请查看服务日志")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// --- basic endpoints ----------------------------------------------------

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"version": Version,
		"time":    store.Now(),
	})
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	free, _ := diskutil.FreeBytes(s.cfg.DataDir)
	active, _ := s.store.CountActiveBuilds()
	writeJSON(w, http.StatusOK, map[string]any{
		"version":               Version,
		"auth_required":         s.cfg.AuthToken != "",
		"archs":                 []string{"x86_64", "aarch64"},
		"max_upload_bytes":      s.cfg.MaxUploadBytes,
		"max_concurrent_builds": s.cfg.MaxConcurrentBuilds,
		"active_builds":         active,
		"storage": map[string]any{
			"enabled":             s.manager.Storage().Kind() == "s3",
			"driver":              s.manager.Storage().Kind(),
			"endpoint":            s.manager.Storage().Endpoint(),
			"bucket":              s.manager.Storage().Bucket(),
			"prefix":              s.manager.Storage().Prefix(),
			"presign_ttl_seconds": int(s.manager.Storage().PresignTTL().Seconds()),
		},
		"disk": map[string]any{
			"data_dir":   s.cfg.DataDir,
			"iso_dir":    s.cfg.IsoDir,
			"work_dir":   s.cfg.WorkDir,
			"builds_dir": s.cfg.BuildDir,
			"free_bytes": free,
		},
		"tools": map[string]any{
			"x86_64":  builder.MissingTools("x86_64"),
			"aarch64": builder.MissingTools("aarch64"),
		},
	})
}

// --- static SPA ---------------------------------------------------------

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		fail(w, http.StatusNotFound, "no_route", "接口不存在: "+r.Method+" "+r.URL.Path)
		return
	}
	index := filepath.Join(s.cfg.WebDir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.Error(w, "前端资源未打包：缺少 "+index, http.StatusNotFound)
		return
	}
	clean := filepath.Clean(strings.TrimPrefix(r.URL.Path, "/"))
	if clean != "." && clean != "" {
		candidate := filepath.Join(s.cfg.WebDir, clean)
		if within(s.cfg.WebDir, candidate) {
			if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
				http.ServeFile(w, r, candidate)
				return
			}
		}
	}
	// SPA fallback: unknown paths are client-side routes.
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeFile(w, r, index)
}

func within(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// --- helpers ------------------------------------------------------------

// Envelope is the wire format the SoybeanAdmin stack uses: every JSON response
// is {code, message, data}, over HTTP 2xx, and business failures come back as
// HTTP 200 with a non-success code, so the stock @sa/axios request layer needs
// no customization at all beyond VITE_SERVICE_SUCCESS_CODE=200.
//
// Biz codes (invalid_hosts, storage_unreachable, offset_mismatch, ...) are
// finer-grained than the bare HTTP status so the UI can branch on them.
type envelope struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data"`
	Details any    `json:"details,omitempty"`
}

// codeSuccess is the envelope code the frontend treats as a successful response.
const codeSuccess = "200"

func fail(w http.ResponseWriter, status int, code, message string, details ...any) {
	body := envelope{
		Code:    code,
		Message: message,
		Data:    map[string]any{},
	}
	if len(details) > 0 && details[0] != nil {
		body.Details = details[0]
	}
	// Errors are flattened to HTTP 200: the business layer reports them via the
	// envelope code, leaving HTTP statuses for transport/boundary failures only.
	_ = status
	writeJSONBody(w, http.StatusOK, body)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	if status == http.StatusNoContent || body == nil {
		w.WriteHeader(status)
		return
	}
	if status >= 200 && status < 300 {
		body = envelope{Code: codeSuccess, Message: "ok", Data: body}
	}
	writeJSONBody(w, http.StatusOK, body)
}

func writeJSONBody(w http.ResponseWriter, status int, body any) {
	buf, err := json.Marshal(body)
	if err != nil {
		// Marshal-to-buffer first so an encoding failure can never send half a
		// payload followed by an abandoned connection.
		buf = []byte(`{"code":"internal","message":"序列化响应失败","data":{}}`)
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write(buf)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(len(buf)))
	w.WriteHeader(status)
	_, _ = w.Write(buf)
}

// decodeJSON reads and validates a JSON request body.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Body == nil {
		fail(w, http.StatusBadRequest, "invalid_body", "请求体为空")
		return false
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		fail(w, http.StatusBadRequest, "invalid_body", "请求体不是合法 JSON: "+err.Error())
		return false
	}
	return true
}

func errStatus(err error) int {
	if errors.Is(err, store.ErrNotFound) {
		return http.StatusNotFound
	}
	return http.StatusInternalServerError
}

// storeFail maps store errors onto API errors.
func storeFail(w http.ResponseWriter, err error, what string) {
	if errors.Is(err, store.ErrNotFound) {
		fail(w, http.StatusNotFound, "not_found", what+"不存在或已被删除")
		return
	}
	fail(w, http.StatusInternalServerError, "internal", what+"查询失败: "+err.Error())
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, "\x00", "")
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return "unnamed"
	}
	return name
}
