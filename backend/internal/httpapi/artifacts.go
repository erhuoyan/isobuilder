package httpapi

import (
	"net/http"
	"os"
	"time"
)

// handleGetArtifact returns an artifact, refreshing its download URL. Presigned
// URLs expire, so the UI calls this before handing the link to a colleague.
func (s *Server) handleGetArtifact(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetArtifact(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "产物")
		return
	}
	if a.Storage == "s3" && a.StorageKey != "" {
		url, expires, err := s.manager.Storage().URL(r.Context(), a.StorageKey)
		if err != nil {
			fail(w, http.StatusBadGateway, "storage_error", "刷新下载链接失败: "+err.Error())
			return
		}
		expiresAt := rfc3339OrEmpty(expires)
		if err := s.store.RefreshArtifactURL(a.ID, url, expiresAt); err != nil {
			s.log.Warn("persist refreshed artifact url", "artifact", a.ID, "err", err)
		}
		a.URL = url
		a.URLExpiresAt = expiresAt
	}
	writeJSON(w, http.StatusOK, map[string]any{"artifact": a})
}

// handleDownloadArtifact redirects to the object store, or streams the file when
// no object storage is configured. The redirect keeps the server out of the data
// path for multi-gigabyte downloads.
func (s *Server) handleDownloadArtifact(w http.ResponseWriter, r *http.Request) {
	a, err := s.store.GetArtifact(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "产物")
		return
	}

	if a.Storage == "s3" && a.StorageKey != "" {
		url := a.URL
		if url == "" || expired(a.URLExpiresAt) {
			fresh, expires, err := s.manager.Storage().URL(r.Context(), a.StorageKey)
			if err != nil {
				fail(w, http.StatusBadGateway, "storage_error", "生成下载链接失败: "+err.Error())
				return
			}
			url = fresh
			_ = s.store.RefreshArtifactURL(a.ID, url, rfc3339OrEmpty(expires))
		}
		http.Redirect(w, r, url, http.StatusFound)
		return
	}

	if a.LocalPath == "" {
		fail(w, http.StatusGone, "artifact_gone", "产物文件已被清理，请重新构建")
		return
	}
	f, err := os.Open(a.LocalPath)
	if err != nil {
		fail(w, http.StatusGone, "artifact_gone", "产物文件不存在: "+err.Error())
		return
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+a.Filename+`"`)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeContent(w, r, a.Filename, fi.ModTime(), f)
}

// expired reports whether a stored URL expiry has passed. An empty value means
// the URL does not expire (public base URL) and is therefore never stale.
func expired(ts string) bool {
	if ts == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return true
	}
	return time.Now().After(t.Add(-5 * time.Minute))
}

func rfc3339OrEmpty(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
