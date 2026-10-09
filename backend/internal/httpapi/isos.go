package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"isobuilder/internal/config"
	"isobuilder/internal/model"
	"isobuilder/internal/settings"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// chunkSize is the upload chunk size advertised to clients. ISO images are tens
// of gigabytes, so uploads are streamed straight to disk and never buffered.
const chunkSize = 64 << 20

// upload is one in-flight ISO upload.
type upload struct {
	ID       string
	IsoID    string
	Name     string
	Filename string
	Arch     string
	Path     string
	Size     int64 // declared size, 0 when unknown
	Received int64
	Created  time.Time

	mu sync.Mutex
}

// uploadRegistry tracks in-flight chunked uploads.
type uploadRegistry struct {
	dir  string
	mu   sync.Mutex
	byID map[string]*upload
}

func newUploadRegistry(dir string) *uploadRegistry {
	return &uploadRegistry{dir: dir, byID: map[string]*upload{}}
}

func (g *uploadRegistry) add(u *upload) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.byID[u.ID] = u
}

func (g *uploadRegistry) get(id string) (*upload, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	u, ok := g.byID[id]
	return u, ok
}

func (g *uploadRegistry) remove(id string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.byID, id)
}

// handleStartUpload creates an upload slot and an ISO record in "uploading".
func (s *Server) handleStartUpload(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string `json:"name"`
		Filename  string `json:"filename"`
		Arch      string `json:"arch"`
		SizeBytes int64  `json:"size_bytes"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Filename = sanitizeFilename(req.Filename)
	if req.Filename == "" {
		fail(w, http.StatusBadRequest, "invalid_filename", "缺少 filename")
		return
	}
	if !strings.HasSuffix(strings.ToLower(req.Filename), ".iso") {
		fail(w, http.StatusBadRequest, "invalid_filename", "只接受 .iso 文件")
		return
	}
	if req.Arch != model.ArchX86 && req.Arch != model.ArchARM {
		fail(w, http.StatusBadRequest, "invalid_arch", `arch 必须是 "x86_64" 或 "aarch64"`)
		return
	}
	if req.SizeBytes > s.cfg.MaxUploadBytes {
		fail(w, http.StatusRequestEntityTooLarge, "too_large",
			fmt.Sprintf("文件 %s 超过单文件上限 %s", humanBytes(req.SizeBytes), humanBytes(s.cfg.MaxUploadBytes)))
		return
	}
	if req.Name == "" {
		req.Name = strings.TrimSuffix(req.Filename, filepath.Ext(req.Filename))
	}

	isoID := store.NewID()
	uploadID := store.NewID()
	path := filepath.Join(s.cfg.IsoDir, isoID+"-"+req.Filename)

	iso := model.ISO{
		ID:        isoID,
		Name:      req.Name,
		Filename:  req.Filename,
		Arch:      req.Arch,
		SizeBytes: req.SizeBytes,
		Status:    model.IsoUploading,
		Source:    model.SourceUploaded,
		Path:      path,
		CreatedAt: store.Now(),
	}
	if err := s.store.CreateISO(iso); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "创建镜像记录失败: "+err.Error())
		return
	}
	u := &upload{
		ID: uploadID, IsoID: isoID, Name: req.Name, Filename: req.Filename,
		Arch: req.Arch, Path: path, Size: req.SizeBytes, Created: time.Now(),
	}
	s.uploads.add(u)

	writeJSON(w, http.StatusCreated, map[string]any{
		"iso":        iso,
		"upload_id":  uploadID,
		"chunk_size": chunkSize,
	})
}

// handleAppendUpload appends one chunk. Chunks must arrive strictly in order;
// a mismatch reports the authoritative offset so the client can resume.
func (s *Server) handleAppendUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("upload_id")
	u, ok := s.uploads.get(id)
	if !ok {
		fail(w, http.StatusNotFound, "upload_not_found", "上传会话不存在或已结束")
		return
	}

	offset, err := parseIntQuery(r, "offset")
	if err != nil || offset < 0 {
		fail(w, http.StatusBadRequest, "invalid_offset", "缺少或非法的 offset 参数")
		return
	}

	u.mu.Lock()
	defer u.mu.Unlock()
	if offset != u.Received {
		fail(w, http.StatusConflict, "offset_mismatch",
			fmt.Sprintf("服务器已接收 %d 字节，本次 offset 为 %d", u.Received, offset),
			map[string]any{"received_bytes": u.Received})
		return
	}
	if u.Size > 0 && u.Received >= u.Size {
		fail(w, http.StatusConflict, "upload_complete", "文件已接收完毕，请调用 finish")
		return
	}

	remaining := s.cfg.MaxUploadBytes - u.Received
	body := io.Reader(r.Body)
	if remaining <= 0 {
		fail(w, http.StatusRequestEntityTooLarge, "too_large", "超过单文件上限")
		return
	}
	if maxChunk := int64(chunkSize) * 2; remaining > maxChunk {
		remaining = maxChunk
	}
	body = io.LimitReader(r.Body, remaining)

	f, err := os.OpenFile(u.Path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "写入镜像文件失败: "+err.Error())
		return
	}
	written, copyErr := io.Copy(f, body)
	closeErr := f.Close()
	if copyErr != nil {
		_ = os.Truncate(u.Path, u.Received)
		fail(w, http.StatusInternalServerError, "internal", "接收数据失败: "+copyErr.Error())
		return
	}
	if closeErr != nil {
		fail(w, http.StatusInternalServerError, "internal", "落盘失败: "+closeErr.Error())
		return
	}
	if written == 0 {
		fail(w, http.StatusBadRequest, "empty_chunk", "分片为空")
		return
	}
	u.Received += written

	writeJSON(w, http.StatusOK, map[string]any{
		"received_bytes": u.Received,
		"complete":       u.Size > 0 && u.Received >= u.Size,
	})
}

// handleFinishUpload seals the file and starts hashing in the background.
func (s *Server) handleFinishUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("upload_id")
	u, ok := s.uploads.get(id)
	if !ok {
		fail(w, http.StatusNotFound, "upload_not_found", "上传会话不存在或已结束")
		return
	}
	u.mu.Lock()
	size := u.Received
	path, isoID := u.Path, u.IsoID
	u.mu.Unlock()

	if size == 0 {
		_ = os.Remove(path)
		s.uploads.remove(id)
		_ = s.store.DeleteISO(isoID)
		fail(w, http.StatusBadRequest, "empty_upload", "没有收到任何数据")
		return
	}
	f, err := os.Open(path)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "打开镜像文件失败: "+err.Error())
		return
	}
	if err := f.Sync(); err != nil {
		s.log.Warn("fsync failed", "path", path, "err", err)
	}
	_ = f.Close()

	if err := s.store.FinishISO(isoID, model.IsoHashing, "", size, ""); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "更新镜像状态失败: "+err.Error())
		return
	}
	s.uploads.remove(id)
	go s.hashISO(isoID, path)

	iso, err := s.store.GetISO(isoID)
	if err != nil {
		storeFail(w, err, "镜像")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"iso": iso})
}

// handleCancelUpload drops an in-flight upload and its partial file.
func (s *Server) handleCancelUpload(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("upload_id")
	u, ok := s.uploads.get(id)
	if !ok {
		fail(w, http.StatusNotFound, "upload_not_found", "上传会话不存在或已结束")
		return
	}
	s.uploads.remove(id)
	_ = os.Remove(u.Path)
	_ = s.store.DeleteISO(u.IsoID)
	w.WriteHeader(http.StatusNoContent)
}

// hashISO computes the artifact checksum of a stored base image. Hashing is
// deliberately deferred: it reads the whole image, which takes minutes.
func (s *Server) hashISO(isoID, path string) {
	sum, size, err := sha256File(path)
	if err != nil {
		// Permission problems read as data problems unless spelled out.
		reason := "计算校验和失败: " + err.Error()
		if os.IsPermission(err) {
			reason = "文件不可读（权限不足）: " + path + "，请 chown " + serviceUserName() + ":" + serviceUserName() + " 或 chmod 0644"
		}
		_ = s.store.FinishISO(isoID, model.IsoError, "", 0, reason)
		return
	}
	if err := s.store.FinishISO(isoID, model.IsoReady, sum, size, ""); err != nil {
		s.log.Error("record iso hash", "iso", isoID, "err", err)
	}
}

// handleListISOs returns the ISO library.
func (s *Server) handleListISOs(w http.ResponseWriter, r *http.Request) {
	isos, err := s.store.ListISOs()
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "查询镜像库失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"isos": isos})
}

// registerRequest describes the three ways a base image enters the library:
// the server's own disk, an object storage address, or a browser upload
// (handled separately by the chunked upload endpoints).
type registerRequest struct {
	Source string `json:"source"` // "path" (default) | "s3"
	Path   string `json:"path"`
	Name   string `json:"name"`
	Arch   string `json:"arch"`

	S3 *struct {
		URL       string `json:"url"` // s3://bucket/key
		Endpoint  string `json:"endpoint"`
		Region    string `json:"region"`
		Bucket    string `json:"bucket"`
		Key       string `json:"key"`
		AccessKey string `json:"access_key"`
		SecretKey string `json:"secret_key"`
		PathStyle *bool  `json:"path_style"`
	} `json:"s3"`
}

// handleRegisterISO adopts an ISO that is already reachable from the server:
// a file on its disk, or an object in an S3-compatible bucket.
func (s *Server) handleRegisterISO(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Source == "" {
		req.Source = "path"
	}
	if req.Arch != model.ArchX86 && req.Arch != model.ArchARM {
		fail(w, http.StatusBadRequest, "invalid_arch", `arch 必须是 "x86_64" 或 "aarch64"`)
		return
	}
	switch req.Source {
	case "path":
		s.registerFromPath(w, req)
	case "s3":
		s.registerFromS3(w, r, req)
	default:
		fail(w, http.StatusBadRequest, "invalid_source", `source 必须是 "path" 或 "s3"`)
	}
}

// registerFromPath adopts a file already sitting on the server.
func (s *Server) registerFromPath(w http.ResponseWriter, req registerRequest) {
	path := strings.TrimSpace(req.Path)
	if path == "" {
		fail(w, http.StatusBadRequest, "invalid_path", "缺少 path")
		return
	}
	if !filepath.IsAbs(path) {
		fail(w, http.StatusBadRequest, "invalid_path", "path 必须是服务器上的绝对路径")
		return
	}
	path = filepath.Clean(path)
	if !allowedISOPath(s.cfg, path) {
		fail(w, http.StatusBadRequest, "invalid_path",
			"只能登记数据目录内的镜像，请先把文件放到 "+s.cfg.IsoDir)
		return
	}
	fi, err := os.Stat(path)
	if err != nil || fi.IsDir() {
		fail(w, http.StatusBadRequest, "invalid_path", "路径不存在或不是文件: "+path)
		return
	}
	// Check readability now rather than during hashing: an ISO dropped in by
	// root with restrictive permissions would otherwise look registered, then
	// fail minutes later with a message about checksums.
	if err := readableByService(path); err != nil {
		fail(w, http.StatusBadRequest, "path_unreadable", err.Error())
		return
	}

	iso := model.ISO{
		ID:        store.NewID(),
		Name:      isoName(req.Name, filepath.Base(path)),
		Filename:  filepath.Base(path),
		Arch:      req.Arch,
		SizeBytes: fi.Size(),
		Status:    model.IsoHashing,
		Source:    model.SourceRegistered,
		Path:      path,
		CreatedAt: store.Now(),
	}
	if err := s.store.CreateISO(iso); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "登记镜像失败: "+err.Error())
		return
	}
	go s.hashISO(iso.ID, iso.Path)
	writeJSON(w, http.StatusCreated, map[string]any{"iso": iso})
}

// registerFromS3 pulls an ISO from an object store into the local image
// directory. Explicit fields override the system settings, so a one-off image in
// someone else's bucket works without touching the global configuration.
func (s *Server) registerFromS3(w http.ResponseWriter, r *http.Request, req registerRequest) {
	if req.S3 == nil {
		fail(w, http.StatusBadRequest, "invalid_s3", "缺少 s3 配置")
		return
	}
	cfg := settings.Load(s.store, s.cfg.Storage).ToConfig()

	bucket, key := strings.TrimSpace(req.S3.Bucket), strings.TrimSpace(req.S3.Key)
	if raw := strings.TrimSpace(req.S3.URL); raw != "" {
		parsed, err := parseS3URL(raw)
		if err != nil {
			fail(w, http.StatusBadRequest, "invalid_s3", err.Error())
			return
		}
		if bucket == "" {
			bucket = parsed.Bucket
		}
		if key == "" {
			key = parsed.Key
		}
	}
	if endpoint := strings.TrimSpace(req.S3.Endpoint); endpoint != "" {
		cfg.Endpoint = endpoint
	}
	if region := strings.TrimSpace(req.S3.Region); region != "" {
		cfg.Region = region
	}
	if ak := strings.TrimSpace(req.S3.AccessKey); ak != "" {
		cfg.AccessKey = ak
	}
	if sk := strings.TrimSpace(req.S3.SecretKey); sk != "" {
		cfg.SecretKey = sk
	}
	if req.S3.PathStyle != nil {
		cfg.PathStyle = *req.S3.PathStyle
	}
	if bucket == "" || key == "" {
		fail(w, http.StatusBadRequest, "invalid_s3",
			"需要提供对象地址（s3://bucket/path/to.iso），或同时给出 bucket 与 key")
		return
	}
	cfg.Bucket = bucket
	if cfg.AccessKey == "" || cfg.SecretKey == "" {
		fail(w, http.StatusBadRequest, "storage_not_configured",
			"未提供对象存储凭据，请先在「系统配置」里填写，或在本表单里临时填写 AK/SK")
		return
	}

	client := storage.NewS3(cfg)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	probeErr := client.Head(ctx, key)
	cancel()
	if probeErr != nil {
		fail(w, http.StatusBadRequest, "s3_object_unreachable", probeErr.Error())
		return
	}

	filename := path.Base(key)
	iso := model.ISO{
		ID:        store.NewID(),
		Name:      isoName(req.Name, filename),
		Filename:  filename,
		Arch:      req.Arch,
		Status:    model.IsoDownloading,
		Source:    model.SourceS3,
		Path:      filepath.Join(s.cfg.IsoDir, store.NewID()+"-"+filename),
		CreatedAt: store.Now(),
	}
	if err := s.store.CreateISO(iso); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "登记镜像失败: "+err.Error())
		return
	}
	go s.fetchISOFromS3(iso.ID, cfg, key)

	writeJSON(w, http.StatusCreated, map[string]any{"iso": iso})
}

// fetchISOFromS3 downloads the object and then hashes it, mirroring the states
// of an upload (uploading/hashing → ready/error).
func (s *Server) fetchISOFromS3(isoID string, cfg config.StorageConfig, key string) {
	iso, err := s.store.GetISO(isoID)
	if err != nil {
		s.log.Error("load iso for s3 fetch", "iso", isoID, "err", err)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	client := storage.NewS3(cfg)
	size, err := client.Get(ctx, key, iso.Path, nil)
	if err != nil {
		s.log.Error("fetch iso from s3", "iso", isoID, "key", key, "err", err)
		_ = s.store.FinishISO(isoID, model.IsoError, "", 0, "从对象存储下载失败: "+err.Error())
		return
	}
	s.log.Info("fetched iso from s3", "iso", isoID, "key", key, "bytes", size)
	_ = s.store.FinishISO(isoID, model.IsoHashing, "", size, "")
	s.hashISO(isoID, iso.Path)
}

// s3Target is a parsed s3://bucket/key address.
type s3Target struct {
	Bucket string
	Key    string
}

func parseS3URL(raw string) (s3Target, error) {
	trimmed := strings.TrimPrefix(strings.TrimSpace(raw), "s3://")
	// A plain https URL to an S3 endpoint is accepted too: the first path
	// segment is the bucket under path-style addressing.
	if trimmed == raw {
		trimmed = strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
		if idx := strings.Index(trimmed, "/"); idx > 0 {
			rest := strings.TrimPrefix(trimmed[idx+1:], "/")
			parts := strings.SplitN(rest, "/", 2)
			if len(parts) == 2 {
				// host/bucket/key — the host is the endpoint, which the caller
				// must supply separately unless it is the system default.
				return s3Target{Bucket: parts[0], Key: parts[1]}, nil
			}
		}
		return s3Target{}, fmt.Errorf("无法从 %q 解析对象地址，请改用 s3://bucket/key 格式", raw)
	}
	parts := strings.SplitN(strings.TrimPrefix(trimmed, "/"), "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return s3Target{}, fmt.Errorf("对象地址 %q 格式不正确，应为 s3://bucket/path/to.iso", raw)
	}
	return s3Target{Bucket: parts[0], Key: parts[1]}, nil
}

// readableByService reports whether the running service account can open a file,
// with an error message that names the actual fix. Files are commonly copied in
// as root, which leaves them unreadable for the service user.
func readableByService(path string) error {
	f, err := os.Open(path)
	if err == nil {
		_ = f.Close()
		return nil
	}
	if os.IsPermission(err) {
		return fmt.Errorf("文件存在，但服务用户（%s）读不到它。执行其中一条即可：chown %s:%s %s   或   chmod 0644 %s",
			serviceUserName(), serviceUserName(), serviceUserName(), path, path)
	}
	return fmt.Errorf("无法读取 %s: %w", path, err)
}

// serviceUserName is the account the service runs as, used in operator hints.
func serviceUserName() string {
	if u := os.Getenv("ISOBUILDER_SERVICE_USER"); u != "" {
		return u
	}
	return "isobuilder"
}

// isoName derives a display name when none was given.
func isoName(given, filename string) string {
	if name := strings.TrimSpace(given); name != "" {
		return name
	}
	return strings.TrimSuffix(filename, path.Ext(filename))
}

// handleDeleteISO removes an image from the library. Uploaded files are deleted
// from disk; registered files are only unlinked from the library.
func (s *Server) handleDeleteISO(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	iso, err := s.store.GetISO(id)
	if err != nil {
		storeFail(w, err, "镜像")
		return
	}
	used, err := s.store.CountProjectsUsingISO(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if used > 0 {
		fail(w, http.StatusConflict, "iso_in_use",
			fmt.Sprintf("还有 %d 个项目在使用该镜像，请先删除或改绑这些项目", used))
		return
	}
	if iso.Source == model.SourceUploaded && within(s.cfg.IsoDir, iso.Path) {
		if err := os.Remove(iso.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			fail(w, http.StatusInternalServerError, "internal", "删除镜像文件失败: "+err.Error())
			return
		}
	}
	if err := s.store.DeleteISO(id); err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// allowedISOPath restricts registration to the managed data directory, so the
// API cannot be used to make the server read arbitrary files off the host.
func allowedISOPath(cfg config.Config, path string) bool {
	return within(cfg.DataDir, path)
}
