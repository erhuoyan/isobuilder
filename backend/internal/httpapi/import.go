package httpapi

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"isobuilder/internal/hosts"
	"isobuilder/internal/model"
	"isobuilder/internal/store"
)

// maxArchiveBytes bounds an uploaded project package and every member inside it.
const maxArchiveBytes = 64 << 20

// archiveEntry is one extracted file of a project package.
type archiveEntry struct {
	Name string
	Body []byte
}

// handleImportProject creates a project from an uploaded archive containing a
// kickstart and a host inventory. The operator exports exactly what they built
// by hand for the shell scripts, so path conventions inside the archive are
// unknown: both files are located by content, not position.
func (s *Server) handleImportProject(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxArchiveBytes); err != nil {
		fail(w, http.StatusBadRequest, "invalid_body", "无法解析上传内容: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "missing_file", "缺少 file 字段（项目包 zip / tar.gz）")
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxArchiveBytes+1))
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "读取上传内容失败: "+err.Error())
		return
	}
	if len(data) == 0 {
		fail(w, http.StatusBadRequest, "empty_file", "上传内容为空")
		return
	}
	if len(data) > maxArchiveBytes {
		fail(w, http.StatusRequestEntityTooLarge, "too_large", "项目包超过 64 MiB")
		return
	}

	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = strings.TrimSuffix(sanitizeFilename(header.Filename), path.Ext(sanitizeFilename(header.Filename)))
		name = strings.TrimSuffix(name, ".tar")
	}

	entries, err := extractArchive(data, sanitizeFilename(header.Filename))
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid_archive", "解压项目包失败: "+err.Error())
		return
	}

	detected := map[string]string{}
	var ksBody, hostsBody []byte
	var ksName, hostsName string

	// A kickstart declares %packages or %pre; an inventory parses as hosts.
	for _, e := range entries {
		base := path.Base(e.Name)
		if strings.HasPrefix(base, ".") {
			continue
		}
		lower := strings.ToLower(base)
		if ksBody == nil && strings.HasSuffix(lower, ".ks") {
			ksBody, ksName = e.Body, e.Name
			continue
		}
		if hostsBody == nil && looksLikeInventory(lower) && hosts.Parse(string(e.Body), "auto").OK {
			hostsBody, hostsName = e.Body, e.Name
		}
	}

	if ksBody == nil {
		fail(w, http.StatusBadRequest, "archive_no_kickstart",
			"项目包里没有找到 kickstart 文件（*.ks）")
		return
	}
	if hostsBody == nil {
		fail(w, http.StatusBadRequest, "archive_no_hosts",
			"项目包里没有找到可解析的主机清单（hosts*.yaml / hosts*.yml / hosts*.json）")
		return
	}
	detected["kickstart"] = ksName
	detected["hosts"] = hostsName

	// The kickstart is stored as a normal template so it can be reused and
	// previewed like any other, which is also what the build pipeline reads.
	ks, apiErr := s.storeKickstart(string(ksBody), sanitizeFilename(path.Base(ksName)), "any")
	if apiErr != nil {
		apiErr.write(w)
		return
	}

	req := projectRequest{
		Name:        &name,
		Arch:        strPtr(strings.TrimSpace(r.FormValue("arch"))),
		IsoID:       strPtr(strings.TrimSpace(r.FormValue("iso_id"))),
		KickstartID: &ks.ID,
		HostsRaw:    strPtr(string(hostsBody)),
		HostsFormat: strPtr("auto"),
	}
	if desc := strings.TrimSpace(r.FormValue("description")); desc != "" {
		req.Description = &desc
	}
	p, apiErr := s.buildProject(req, nil)
	if apiErr != nil {
		// The template was stored for a project that cannot be created; drop it
		// so a failed import does not leave litter in the template library.
		if !ks.Builtin {
			_ = s.store.DeleteKickstart(ks.ID)
			_ = removeFileIfInside(s.cfg.KickDir, ks.Path)
		}
		apiErr.write(w)
		return
	}
	p.ID = store.NewID()
	p.CreatedAt = store.Now()
	p.UpdatedAt = p.CreatedAt
	if err := s.store.SaveProject(p, true); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "保存项目失败: "+err.Error())
		return
	}
	out, err := s.store.GetProject(p.ID)
	if err != nil {
		storeFail(w, err, "项目")
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"project": out, "detected": detected})
}

// looksLikeInventory filters archive members that could plausibly hold hosts.
func looksLikeInventory(lowerBase string) bool {
	for _, suffix := range []string{".yaml", ".yml", ".json"} {
		if strings.HasSuffix(lowerBase, suffix) {
			return true
		}
	}
	return false
}

// extractArchive reads a zip or tar (optionally gzipped) into memory. Entries
// with absolute paths or traversal segments are rejected outright.
func extractArchive(data []byte, filename string) ([]archiveEntry, error) {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		return readZip(data)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"), strings.HasSuffix(lower, ".tar"):
		return readTar(data)
	default:
		// Sniff the magic bytes so a misnamed archive still works.
		if bytes.HasPrefix(data, []byte("PK\x03\x04")) {
			return readZip(data)
		}
		return readTar(data)
	}
}

func readZip(data []byte) ([]archiveEntry, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	entries := make([]archiveEntry, 0, len(zr.File))
	var total int64
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}
		name, skip, err := safeArchivePath(f.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if total += int64(f.UncompressedSize64); total > maxArchiveBytes {
			return nil, errors.New("解压后总大小超过 64 MiB")
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(rc, maxArchiveBytes+1))
		rc.Close()
		if err != nil {
			return nil, err
		}
		entries = append(entries, archiveEntry{Name: name, Body: body})
	}
	return entries, nil
}

func readTar(data []byte) ([]archiveEntry, error) {
	var reader io.Reader = bytes.NewReader(data)
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		reader = gz
	}
	tr := tar.NewReader(reader)
	entries := []archiveEntry{}
	var total int64
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
			continue
		}
		name, skip, err := safeArchivePath(hdr.Name)
		if err != nil {
			return nil, err
		}
		if skip {
			continue
		}
		if total += hdr.Size; total > maxArchiveBytes {
			return nil, errors.New("解压后总大小超过 64 MiB")
		}
		body, err := io.ReadAll(io.LimitReader(tr, maxArchiveBytes+1))
		if err != nil {
			return nil, err
		}
		entries = append(entries, archiveEntry{Name: name, Body: body})
	}
	return entries, nil
}

// safeArchivePath rejects absolute paths and traversal, and flags macOS metadata
// for skipping. Traversal matters here: a crafted archive must not be able to
// make the importer write outside the data directory.
func safeArchivePath(name string) (cleaned string, skip bool, err error) {
	cleaned = path.Clean(strings.ReplaceAll(name, "\\", "/"))
	if path.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return "", false, fmt.Errorf("项目包内包含非法路径: %s", name)
	}
	if cleaned == "." || strings.HasPrefix(cleaned, "__MACOSX/") || strings.Contains(cleaned, "/__MACOSX/") {
		return "", true, nil
	}
	return cleaned, false, nil
}

// storeKickstart persists a template body, shared by upload and import.
func (s *Server) storeKickstart(body, name, arch string) (model.Kickstart, *apiErr) {
	if strings.TrimSpace(body) == "" {
		return model.Kickstart{}, newAPIErr(http.StatusBadRequest, "empty_kickstart", "kickstart 内容为空")
	}
	if !strings.Contains(body, hostsPlaceholder) {
		return model.Kickstart{}, newAPIErr(http.StatusBadRequest, "kickstart_missing_placeholder",
			"kickstart 缺少 "+hostsPlaceholder+" 占位符，无法注入主机清单")
	}
	if _, err := os.Stat(s.cfg.KickDir); err != nil {
		return model.Kickstart{}, newAPIErr(http.StatusInternalServerError, "internal", "模板目录不可用: "+err.Error())
	}
	id := store.NewID()
	// Same layout as ISOs: kickstarts/<id>/<filename>, so the file keeps its
	// original name instead of being prefixed with the id.
	dest := store.ISOPath(s.cfg.DataDir, model.ISO{
		Path: store.ManagedKickstartRelPath(id, name),
	})
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return model.Kickstart{}, newAPIErr(http.StatusInternalServerError, "internal", "创建模板目录失败: "+err.Error())
	}
	if err := os.WriteFile(dest, []byte(body), 0o644); err != nil {
		return model.Kickstart{}, newAPIErr(http.StatusInternalServerError, "internal", "保存模板失败: "+err.Error())
	}
	k := model.Kickstart{
		ID: id, Name: name, Arch: arch, Builtin: false,
		Path: dest, SizeBytes: int64(len(body)), CreatedAt: store.Now(),
	}
	if err := s.store.CreateKickstart(k); err != nil {
		_ = os.Remove(dest)
		return model.Kickstart{}, newAPIErr(http.StatusInternalServerError, "internal", "登记模板失败: "+err.Error())
	}
	k.Placeholders = uniqueStrings(placeholderRe.FindAllString(body, -1))
	if k.Placeholders == nil {
		k.Placeholders = []string{}
	}
	return k, nil
}

func strPtr(v string) *string { return &v }

// removeFileIfInside deletes a file only when it lives inside root.
func removeFileIfInside(root, path string) error {
	if path == "" || !within(root, path) {
		return nil
	}
	return os.Remove(path)
}
