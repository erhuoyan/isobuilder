package httpapi

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"isobuilder/internal/model"
	"isobuilder/internal/store"
)

// maxKickstartBytes bounds an uploaded template. Kickstarts are scripts; a few
// hundred kilobytes is already unusually large.
const maxKickstartBytes = 2 << 20

// hostsPlaceholder is the token the build pipeline replaces with the host
// inventory JSON. Every usable kickstart must carry it.
const hostsPlaceholder = "%%HOSTS_JSON_PLACEHOLDER%%"

var placeholderRe = regexp.MustCompile(`%%[A-Z0-9_]+%%`)

// handleListKickstarts returns every template, built-ins first.
func (s *Server) handleListKickstarts(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListKickstarts()
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "查询模板失败: "+err.Error())
		return
	}
	out := make([]model.Kickstart, 0, len(list))
	for _, k := range list {
		out = append(out, s.decorateKickstart(k))
	}
	writeJSON(w, http.StatusOK, map[string]any{"kickstarts": out})
}

// decorateKickstart fills in the placeholders found in the template body.
func (s *Server) decorateKickstart(k model.Kickstart) model.Kickstart {
	if body, err := os.ReadFile(k.Path); err == nil {
		k.Placeholders = uniqueStrings(placeholderRe.FindAllString(string(body), -1))
		k.SizeBytes = int64(len(body))
	}
	if k.Placeholders == nil {
		k.Placeholders = []string{}
	}
	return k
}

// handleCreateKickstart stores an uploaded kickstart template.
func (s *Server) handleCreateKickstart(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxKickstartBytes); err != nil {
		fail(w, http.StatusBadRequest, "invalid_body", "无法解析上传内容: "+err.Error())
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		fail(w, http.StatusBadRequest, "missing_file", "缺少 file 字段（.ks 文件）")
		return
	}
	defer file.Close()

	body, err := io.ReadAll(io.LimitReader(file, maxKickstartBytes+1))
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "读取模板失败: "+err.Error())
		return
	}
	if len(body) == 0 {
		fail(w, http.StatusBadRequest, "empty_file", "模板内容为空")
		return
	}
	if len(body) > maxKickstartBytes {
		fail(w, http.StatusRequestEntityTooLarge, "too_large", "模板超过 2 MiB")
		return
	}

	filename := sanitizeFilename(header.Filename)
	if !strings.HasSuffix(strings.ToLower(filename), ".ks") {
		fail(w, http.StatusBadRequest, "invalid_filename", "只接受 .ks 文件")
		return
	}
	arch := strings.TrimSpace(r.FormValue("arch"))
	if arch == "" {
		arch = "any"
	}
	if arch != model.ArchX86 && arch != model.ArchARM && arch != "any" {
		fail(w, http.StatusBadRequest, "invalid_arch", `arch 必须是 "x86_64"、"aarch64" 或 "any"`)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		name = filename
	}

	remark := strings.TrimSpace(r.FormValue("remark"))

	k, apiErr := s.storeKickstart(string(body), filename, arch)
	if apiErr != nil {
		apiErr.write(w)
		return
	}
	if name != filename {
		if err := s.store.UpdateKickstart(k.ID, name, remark); err != nil {
			fail(w, http.StatusInternalServerError, "internal", "保存模板失败: "+err.Error())
			return
		}
		k.Name = name
	}
	k.Remark = remark
	writeJSON(w, http.StatusCreated, map[string]any{"kickstart": k})
}

// handleUpdateKickstart edits the editable metadata (name, remark) and, when a
// body is present, the template content itself. The builtin flag cannot be
// changed and built-in templates keep their files read-only.
func (s *Server) handleUpdateKickstart(w http.ResponseWriter, r *http.Request) {
	k, err := s.store.GetKickstart(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "模板")
		return
	}

	var req struct {
		Name    *string `json:"name"`
		Remark  *string `json:"remark"`
		Content *string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}

	name := k.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			fail(w, http.StatusBadRequest, "invalid_name", "模板名称不能为空")
			return
		}
	}

	if req.Content != nil {
		body := *req.Content
		if strings.TrimSpace(body) == "" {
			fail(w, http.StatusBadRequest, "empty_kickstart", "kickstart 内容为空")
			return
		}
		if len(body) > maxKickstartBytes {
			fail(w, http.StatusRequestEntityTooLarge, "too_large", "模板超过 2 MiB")
			return
		}
		if !strings.Contains(body, hostsPlaceholder) {
			fail(w, http.StatusBadRequest, "kickstart_missing_placeholder",
				"kickstart 缺少 "+hostsPlaceholder+" 占位符，无法注入主机清单")
			return
		}
		if err := os.WriteFile(k.Path, []byte(body), 0o644); err != nil {
			fail(w, http.StatusInternalServerError, "internal", "保存模板失败: "+err.Error())
			return
		}
	}

	if err := s.store.UpdateKickstart(k.ID, name, pickRemark(req.Remark, k.Remark)); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "保存模板失败: "+err.Error())
		return
	}

	out := s.decorateKickstart(k)
	out.Name = name
	out.Remark = pickRemark(req.Remark, k.Remark)
	writeJSON(w, http.StatusOK, map[string]any{"kickstart": out})
}

func pickRemark(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return strings.TrimSpace(*v)
}

// handleKickstartContent returns the template body for in-browser preview.
func (s *Server) handleKickstartContent(w http.ResponseWriter, r *http.Request) {
	k, err := s.store.GetKickstart(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "模板")
		return
	}
	body, err := os.ReadFile(k.Path)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "读取模板失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"content": string(body)})
}

// handleDeleteKickstart deletes a user template. Built-in templates are
// immutable so the image always ships a working default.
func (s *Server) handleDeleteKickstart(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	k, err := s.store.GetKickstart(id)
	if err != nil {
		storeFail(w, err, "模板")
		return
	}
	if k.Builtin {
		fail(w, http.StatusConflict, "builtin_kickstart", "内置模板不可删除；如需改动请上传一份新模板")
		return
	}
	used, err := s.store.CountProjectsUsingKickstart(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if used > 0 {
		fail(w, http.StatusConflict, "kickstart_in_use",
			fmt.Sprintf("还有 %d 个项目在使用该模板，请先删除或改绑这些项目", used))
		return
	}
	if within(s.cfg.KickDir, k.Path) {
		if err := os.Remove(k.Path); err != nil && !errors.Is(err, os.ErrNotExist) {
			fail(w, http.StatusInternalServerError, "internal", "删除模板文件失败: "+err.Error())
			return
		}
	}
	if err := s.store.DeleteKickstart(id); err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RegisterBuiltinKickstarts imports the templates baked into the image on first
// start, so a fresh deployment can build an ISO without any upload.
func (s *Server) RegisterBuiltinKickstarts() {
	entries, err := os.ReadDir(s.cfg.BuiltinDir)
	if err != nil {
		s.log.Info("no built-in kickstart directory", "dir", s.cfg.BuiltinDir)
		return
	}
	existing, err := s.store.ListKickstarts()
	if err != nil {
		s.log.Error("list kickstarts", "err", err)
		return
	}
	known := map[string]bool{}
	for _, k := range existing {
		known[k.Name] = true
	}

	names := []string{}
	for _, e := range entries {
		name := e.Name()
		// macOS AppleDouble sidecars (._foo.ks) and dotfiles are archive/lookup
		// litter, and a template without the injection placeholder can never
		// produce an image. Neither belongs in the template library.
		if e.IsDir() || strings.HasPrefix(name, ".") || !strings.HasSuffix(name, ".ks") {
			continue
		}
		body, err := os.ReadFile(filepath.Join(s.cfg.BuiltinDir, name))
		if err != nil || !strings.Contains(string(body), hostsPlaceholder) {
			s.log.Warn("跳过不可用的内置模板", "name", name, "reason", "缺少占位符或不可读")
			continue
		}
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if known[name] {
			continue
		}
		path := filepath.Join(s.cfg.BuiltinDir, name)
		fi, err := os.Stat(path)
		if err != nil {
			continue
		}
		k := model.Kickstart{
			ID:        store.NewID(),
			Name:      name,
			Arch:      builtinArch(name),
			Builtin:   true,
			Path:      path,
			SizeBytes: fi.Size(),
			CreatedAt: store.Now(),
		}
		if err := s.store.CreateKickstart(k); err != nil {
			s.log.Error("register built-in kickstart", "name", name, "err", err)
			continue
		}
		s.log.Info("registered built-in kickstart", "name", name, "arch", k.Arch)
	}
}

// builtinArch infers the target architecture from a template filename.
func builtinArch(name string) string {
	lower := strings.ToLower(name)
	switch {
	case strings.Contains(lower, "euler"), strings.Contains(lower, "arm"), strings.Contains(lower, "kylin"):
		return model.ArchARM
	case strings.Contains(lower, "rocky"), strings.Contains(lower, "centos"), strings.Contains(lower, "x86"):
		return model.ArchX86
	default:
		return "any"
	}
}

func uniqueStrings(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, v := range in {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	return out
}
