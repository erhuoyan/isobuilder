package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"isobuilder/internal/config"
	"isobuilder/internal/model"
	"isobuilder/internal/store"
)

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
// the server's own disk, an S3 object address, or a plain HTTP(S) download URL.
// Nothing is ever uploaded through the browser: a 14 GB image would tie up the
// operator's uplink for hours, so the server does the transfer instead.
type registerRequest struct {
	Source string `json:"source"` // "path" (default) | "url"
	Path   string `json:"path"`
	Name   string `json:"name"`
	Remark string `json:"remark"`
	Arch   string `json:"arch"`
	// URL is an HTTP(S) download link, used by source="url". Object storage
	// works here too: buckets expose HTTP URLs (public, presigned, or signed
	// by a CDN), so there is no separate S3 code path.
	URL string `json:"url"`
}

// handleRegisterISO adopts an ISO the server can reach on its own: a file on its
// disk, or any HTTP(S) download link. Images are tens of gigabytes, so they
// never travel through the browser; the operator pastes a link and the server
// streams it into the image directory.
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
	case "url":
		s.registerFromURL(w, r, req)
	default:
		fail(w, http.StatusBadRequest, "invalid_source", `source 必须是 "path" 或 "url"`)
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

	relPath, relErr := store.RelISOPath(s.cfg.DataDir, path)
	if relErr != nil {
		fail(w, http.StatusBadRequest, "invalid_path", relErr.Error())
		return
	}

	iso := model.ISO{
		ID:        store.NewID(),
		Name:      isoName(req.Name, filepath.Base(path)),
		Filename:  filepath.Base(path),
		Arch:      req.Arch,
		Remark:    strings.TrimSpace(req.Remark),
		SizeBytes: fi.Size(),
		Status:    model.IsoHashing,
		Source:    model.SourceRegistered,
		Path:      relPath,
		CreatedAt: store.Now(),
	}
	if err := s.store.CreateISO(iso); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "登记镜像失败: "+err.Error())
		return
	}
	go s.hashISO(iso.ID, path)
	writeJSON(w, http.StatusCreated, map[string]any{"iso": iso})
}

// registerFromURL imports an image from a download link. This covers object
// storage too: an S3-compatible bucket exposes HTTP URLs (public, presigned, or
// signed by the CDN), so one transport serves mirrors, vendor sites, internal
// file servers and buckets alike.
func (s *Server) registerFromURL(w http.ResponseWriter, r *http.Request, req registerRequest) {
	raw := strings.TrimSpace(req.URL)
	if raw == "" {
		fail(w, http.StatusBadRequest, "invalid_url", "缺少 url")
		return
	}

	parsed, err := url.Parse(raw)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		fail(w, http.StatusBadRequest, "invalid_url", "链接必须是 http:// 或 https:// 开头的完整地址")
		return
	}

	// Probe before creating the record: a wrong link should fail in a second,
	// not after a row appears in the library and sits in "downloading" forever.
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	probe, probeErr := s.probeURL(ctx, raw)
	cancel()
	if probeErr != nil {
		fail(w, http.StatusBadRequest, "url_unreachable", probeErr.Error())
		return
	}

	filename := sanitizeFilename(urlFilename(parsed, probe.filename))
	if filename == "" || filename == "." || filename == "/" {
		fail(w, http.StatusBadRequest, "invalid_url",
			"无法从链接推断文件名，请在链接里带上文件名")
		return
	}
	if !strings.HasSuffix(strings.ToLower(filename), ".iso") {
		filename += ".iso"
	}
	if probe.size > 0 && probe.size > s.cfg.MaxUploadBytes {
		fail(w, http.StatusRequestEntityTooLarge, "too_large",
			fmt.Sprintf("远端文件 %s 超过单文件上限 %s",
				humanBytes(probe.size), humanBytes(s.cfg.MaxUploadBytes)))
		return
	}

	isoID := store.NewID()
	iso := model.ISO{
		ID:        isoID,
		Name:      isoName(req.Name, filename),
		Filename:  filename,
		Arch:      req.Arch,
		Remark:    strings.TrimSpace(req.Remark),
		SizeBytes: probe.size,
		Status:    model.IsoDownloading,
		Source:    model.SourceURL,
		Path:      store.ManagedISORelPath(isoID, filename),
		CreatedAt: store.Now(),
	}
	if err := s.store.CreateISO(iso); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "登记镜像失败: "+err.Error())
		return
	}
	go s.fetchISOFromURL(iso.ID, raw)

	writeJSON(w, http.StatusCreated, map[string]any{"iso": iso})
}

// urlProbe is what the pre-flight request tells us about the remote file.
type urlProbe struct {
	size     int64
	filename string
}

// probeURL checks the link is downloadable. It tries HEAD first and falls back
// to a one-byte ranged GET, because plain static file hosts often reject HEAD.
func (s *Server) probeURL(ctx context.Context, raw string) (urlProbe, error) {
	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, raw, nil)
	if err != nil {
		return urlProbe{}, fmt.Errorf("链接格式不正确: %w", err)
	}
	if resp, headErr := http.DefaultClient.Do(headReq); headErr == nil {
		_ = resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			return urlProbe{size: resp.ContentLength, filename: filenameFromDisposition(resp)}, nil
		}
	}

	getReq, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return urlProbe{}, fmt.Errorf("链接格式不正确: %w", err)
	}
	getReq.Header.Set("Range", "bytes=0-0")
	resp, err := http.DefaultClient.Do(getReq)
	if err != nil {
		return urlProbe{}, fmt.Errorf("无法访问该链接: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return urlProbe{}, fmt.Errorf("远端返回 HTTP %d，请确认链接可直接下载", resp.StatusCode)
	}

	var size int64
	// Content-Range looks like "bytes 0-0/12345"; the total follows the slash.
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		if idx := strings.LastIndex(cr, "/"); idx >= 0 {
			if total, convErr := strconv.ParseInt(cr[idx+1:], 10, 64); convErr == nil {
				size = total
			}
		}
	}
	return urlProbe{size: size, filename: filenameFromDisposition(resp)}, nil
}

// filenameFromDisposition prefers the server-provided name, since CDN and
// presigned links often end in an opaque token rather than the real filename.
func filenameFromDisposition(resp *http.Response) string {
	cd := resp.Header.Get("Content-Disposition")
	if cd == "" {
		return ""
	}
	if _, params, err := mime.ParseMediaType(cd); err == nil {
		return params["filename"]
	}
	return ""
}

// urlFilename derives a filename from the URL path, ignoring the query string.
func urlFilename(parsed *url.URL, fromHeader string) string {
	if fromHeader != "" {
		return fromHeader
	}
	return path.Base(parsed.Path)
}

// fetchISOFromURL streams the image to disk and then hashes it, mirroring the
// states of the removed upload path (downloading → hashing → ready/error).
func (s *Server) fetchISOFromURL(isoID, raw string) {
	iso, err := s.store.GetISO(isoID)
	if err != nil {
		s.log.Error("load iso for url fetch", "iso", isoID, "err", err)
		return
	}

	// Images are large; a stalled mirror must not hold the slot forever.
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Hour)
	defer cancel()

	absPath := store.ISOPath(s.cfg.DataDir, iso)
	if mkErr := os.MkdirAll(filepath.Dir(absPath), 0o755); mkErr != nil {
		_ = s.store.FinishISO(isoID, model.IsoError, "", 0, "创建镜像目录失败: "+mkErr.Error())
		return
	}

	size, err := downloadTo(ctx, raw, absPath, s.cfg.MaxUploadBytes)
	if err != nil {
		s.log.Error("fetch iso from url", "iso", isoID, "err", err)
		_ = s.store.FinishISO(isoID, model.IsoError, "", 0, "从链接下载失败: "+err.Error())
		return
	}
	s.log.Info("fetched iso from url", "iso", isoID, "bytes", size)
	_ = s.store.FinishISO(isoID, model.IsoHashing, "", size, "")
	s.hashISO(isoID, absPath)
}

// downloadTo streams src into dest, refusing anything above limit. A partly
// written file is removed on failure so a retry starts from a clean slate.
func downloadTo(ctx context.Context, src, dest string, limit int64) (int64, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src, nil)
	if err != nil {
		return 0, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("远端返回 HTTP %d", resp.StatusCode)
	}

	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return 0, err
	}

	var body io.Reader = resp.Body
	if limit > 0 {
		// Read one byte past the limit so an oversized body is detected rather
		// than silently truncated.
		body = io.LimitReader(resp.Body, limit+1)
	}

	written, copyErr := io.Copy(f, body)
	if closeErr := f.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		_ = os.Remove(dest)
		return 0, copyErr
	}
	if limit > 0 && written > limit {
		_ = os.Remove(dest)
		return 0, fmt.Errorf("文件超过单文件上限 %s", humanBytes(limit))
	}
	return written, nil
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
	// Downloaded files are ours to delete; a registered ISO points at a file
	// the operator put there, so leave it alone.
	absPath := store.ISOPath(s.cfg.DataDir, iso)
	managed := iso.Source == model.SourceURL
	if managed && within(s.cfg.IsoDir, absPath) {
		if err := os.Remove(absPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			fail(w, http.StatusInternalServerError, "internal", "删除镜像文件失败: "+err.Error())
			return
		}
		// Managed ISOs live in their own per-id directory; drop it once empty.
		_ = os.Remove(filepath.Dir(absPath))
	}
	if err := s.store.DeleteISO(id); err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

// allowedISOPath restricts registration to the managed data directory, so the
// API cannot be used to make the server read arbitrary files off the host.
func allowedISOPath(cfg config.Config, path string) bool {
	return within(cfg.DataDir, path)
}
