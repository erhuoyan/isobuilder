package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"isobuilder/internal/hosts"
	"isobuilder/internal/model"
	"isobuilder/internal/store"
)

// maxPostScriptBytes bounds the project-level post-install script. It is
// base64-encoded into the kickstart, so 64 KiB is already generous.
const maxPostScriptBytes = 64 << 10

// projectRequest is the create/update payload. Pointer fields distinguish
// "absent" from "empty" on update.
type projectRequest struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	Arch        *string `json:"arch"`
	IsoID       *string `json:"iso_id"`
	KickstartID *string `json:"kickstart_id"`
	ScriptID    *string `json:"script_id"`
	IsoLabel    *string `json:"iso_label"`
	OutputName  *string `json:"output_name"`
	PostScript  *string `json:"post_script"`
	PayloadMode *string `json:"payload_mode"`
	HostsRaw    *string `json:"hosts_raw"`
	HostsFormat *string `json:"hosts_format"`
}

// handleListProjects returns every project, newest edit first. The raw host
// inventory is omitted because a list of projects does not need megabytes of YAML.
func (s *Server) handleListProjects(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListProjects()
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "查询项目失败: "+err.Error())
		return
	}
	for i := range list {
		list[i].HostsRaw = ""
	}
	writeJSON(w, http.StatusOK, map[string]any{"projects": list})
}

// handleGetProject returns one project with its inventory.
func (s *Server) handleGetProject(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "项目")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": p})
}

// handleCreateProject validates and stores a project.
func (s *Server) handleCreateProject(w http.ResponseWriter, r *http.Request) {
	var req projectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	p, apiErr := s.buildProject(req, nil)
	if apiErr != nil {
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
	writeJSON(w, http.StatusCreated, map[string]any{"project": out})
}

// handleUpdateProject replaces the mutable fields of a project.
func (s *Server) handleUpdateProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	existing, err := s.store.GetProject(id)
	if err != nil {
		storeFail(w, err, "项目")
		return
	}
	var req projectRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	updated, apiErr := s.buildProject(req, &existing)
	if apiErr != nil {
		apiErr.write(w)
		return
	}
	updated.ID = existing.ID
	updated.CreatedAt = existing.CreatedAt
	updated.UpdatedAt = store.Now()
	if err := s.store.SaveProject(updated, false); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "保存项目失败: "+err.Error())
		return
	}
	out, err := s.store.GetProject(id)
	if err != nil {
		storeFail(w, err, "项目")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"project": out})
}

// handleDeleteProject removes a project, refusing while a build is active.
func (s *Server) handleDeleteProject(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetProject(id); err != nil {
		storeFail(w, err, "项目")
		return
	}
	active, err := s.store.CountActiveBuildsForProject(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if active > 0 {
		fail(w, http.StatusConflict, "project_busy", fmt.Sprintf("项目有 %d 个进行中的构建，请先取消", active))
		return
	}
	if err := s.store.DeleteProject(id); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "删除项目失败: "+err.Error())
		return
	}
	// Success must carry the envelope too: the admin SPA treats any body-less
	// response (204) as a backend failure even when the delete happened.
	writeJSON(w, http.StatusOK, map[string]any{})
}

// ResummarizeProjects re-derives every project's hosts summary from the stored
// raw inventory. The summary is a snapshot produced by the validator version
// that saved the project: new summary fields (gateway, disk_gb, …) only appear
// in already-created projects after a re-validation. It runs at startup and
// skips inventories the current validator rejects, so a rule change can never
// hide data that was valid when it was saved.
func (s *Server) ResummarizeProjects() {
	projects, err := s.store.ListProjects()
	if err != nil {
		s.log.Error("回填主机摘要失败：查询项目", "err", err)
		return
	}
	updated := 0
	for _, p := range projects {
		if strings.TrimSpace(p.HostsRaw) == "" {
			continue
		}
		res := hosts.Parse(p.HostsRaw, p.HostsFormat)
		if !res.OK {
			continue
		}
		before, _ := json.Marshal(p.HostsSummary)
		after, _ := json.Marshal(res.Summary)
		if res.Count == p.HostsCount && string(before) == string(after) {
			continue
		}
		if err := s.store.UpdateProjectSummary(p.ID, res.Count, res.Summary); err != nil {
			s.log.Error("回填主机摘要失败", "project", p.ID, "err", err)
			continue
		}
		updated++
	}
	if updated > 0 {
		s.log.Info("已按当前校验器回填主机摘要", "count", updated)
	}
}

// handleValidateHosts validates an inventory without persisting anything. The
// frontend calls this while the operator types, so it must stay cheap.
func (s *Server) handleValidateHosts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Content string `json:"content"`
		Format  string `json:"format"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	res := hosts.Parse(req.Content, req.Format)
	if res.Errors == nil {
		res.Errors = []hosts.Error{}
	}
	writeJSON(w, http.StatusOK, res)
}

// apiErr carries an error payload that is already shaped for the client.
type apiErr struct {
	status  int
	code    string
	message string
	details any
}

func (e *apiErr) write(w http.ResponseWriter) {
	fail(w, e.status, e.code, e.message, e.details)
}

// buildProject validates a request into a project row. When base is non-nil the
// request is treated as a partial update of that project.
func (s *Server) buildProject(req projectRequest, base *model.Project) (model.Project, *apiErr) {
	p := model.Project{HostsSummary: []model.HostSummary{}}
	if base != nil {
		p = *base
	}

	name := pick(req.Name, p.Name)
	name = strings.TrimSpace(name)
	if name == "" {
		return p, newAPIErr(http.StatusBadRequest, "invalid_project", "项目名称不能为空")
	}
	p.Name = name
	p.Description = pick(req.Description, p.Description)

	arch := pick(req.Arch, p.Arch)
	if arch == "" {
		arch = model.ArchX86
	}
	if arch != model.ArchX86 && arch != model.ArchARM {
		return p, newAPIErr(http.StatusBadRequest, "invalid_arch", `arch 必须是 "x86_64" 或 "aarch64"`)
	}
	p.Arch = arch

	isoID := pick(req.IsoID, p.IsoID)
	if isoID == "" {
		return p, newAPIErr(http.StatusBadRequest, "missing_iso", "请选择基础镜像")
	}
	iso, err := s.store.GetISO(isoID)
	if err != nil {
		return p, newAPIErr(http.StatusBadRequest, "invalid_iso", "所选基础镜像不存在")
	}
	if iso.Arch != arch {
		return p, newAPIErr(http.StatusBadRequest, "arch_mismatch", fmt.Sprintf("基础镜像 %s 是 %s 架构，与项目架构 %s 不一致", iso.Filename, iso.Arch, arch))
	}
	if iso.Status == model.IsoError {
		return p, newAPIErr(http.StatusBadRequest, "invalid_iso", "基础镜像校验失败，请重新上传: "+iso.Error)
	}
	p.IsoID = isoID

	ksID := pick(req.KickstartID, p.KickstartID)
	if ksID == "" {
		return p, newAPIErr(http.StatusBadRequest, "missing_kickstart", "请选择 kickstart 模板")
	}
	ks, err := s.store.GetKickstart(ksID)
	if err != nil {
		return p, newAPIErr(http.StatusBadRequest, "invalid_kickstart", "所选 kickstart 模板不存在")
	}
	if ks.Arch != "any" && ks.Arch != arch {
		return p, newAPIErr(http.StatusBadRequest, "kickstart_arch_mismatch", fmt.Sprintf("模板 %s 面向 %s，与项目架构 %s 不一致", ks.Name, ks.Arch, arch))
	}
	p.KickstartID = ksID

	p.IsoLabel = strings.TrimSpace(pick(req.IsoLabel, p.IsoLabel))
	p.OutputName = strings.TrimSpace(pick(req.OutputName, p.OutputName))
	if p.OutputName != "" {
		p.OutputName = sanitizeFilename(p.OutputName)
		if !strings.HasSuffix(strings.ToLower(p.OutputName), ".iso") {
			p.OutputName += ".iso"
		}
	}
	if strings.Contains(p.IsoLabel, " ") {
		return p, newAPIErr(http.StatusBadRequest, "invalid_iso_label", "iso_label 不能包含空格（ISO9660 卷标限制）")
	}
	if len(p.IsoLabel) > 32 {
		return p, newAPIErr(http.StatusBadRequest, "invalid_iso_label", "iso_label 超过 32 个字符")
	}

	// post_script: 省略 = 不变；传值（含空串）= 覆盖。base64 编码注入 KS，这里只限大小。
	postScript := normalizeScriptNewlines(pick(req.PostScript, p.PostScript))
	if len(postScript) > maxPostScriptBytes {
		return p, newAPIErr(http.StatusBadRequest, "invalid_post_script",
			fmt.Sprintf("装机后脚本超过 %d 字节", maxPostScriptBytes))
	}
	p.PostScript = postScript

	// script_id: 引用脚本库。省略 = 不变；空串 = 解除绑定。
	scriptID := pick(req.ScriptID, p.ScriptID)
	if scriptID != "" {
		if _, err := s.store.GetScript(scriptID); err != nil {
			return p, newAPIErr(http.StatusBadRequest, "invalid_script", "所选脚本不存在，请重新选择")
		}
	}
	p.ScriptID = scriptID

	// payload_mode: 可选的 U 盘文件注入。省略 = 不变。
	//   off   —— 不注入（默认，模板里的占位符替换成一行注释）
	//   label —— 只按卷标 ISO_PAYLOAD 找分区（11 字符，FAT32/exFAT 上限）
	//   auto  —— 卷标优先，未命中再扫描非目标盘
	payloadMode := pick(req.PayloadMode, p.PayloadMode)
	if payloadMode == "" {
		payloadMode = "off"
	}
	if payloadMode != "off" && payloadMode != "label" && payloadMode != "auto" {
		return p, newAPIErr(http.StatusBadRequest, "invalid_payload_mode",
			`payload_mode 必须是 "off"、"label" 或 "auto"`)
	}
	p.PayloadMode = payloadMode
	raw := pick(req.HostsRaw, p.HostsRaw)
	format := pick(req.HostsFormat, p.HostsFormat)
	if format == "" || format == "auto" {
		format = detectFormat(raw)
	}
	if format != "yaml" && format != "json" {
		return p, newAPIErr(http.StatusBadRequest, "invalid_format", `hosts_format 必须是 "yaml"、"json" 或 "auto"`)
	}
	res := hosts.Parse(raw, format)
	if !res.OK {
		return p, newAPIErr(http.StatusBadRequest, "invalid_hosts", firstHostError(res)).
			with(map[string]any{"hosts": res.Errors})
	}
	p.HostsRaw = raw
	p.HostsFormat = format
	p.HostsJSON = res.CanonicalJSON
	p.HostsCount = res.Count
	p.HostsSummary = res.Summary
	if p.OutputName == "" {
		p.OutputName = defaultOutputName(iso.Filename)
	}
	return p, nil
}

// newAPIErr builds an error payload for the client.
func newAPIErr(status int, code, message string) *apiErr {
	return &apiErr{status: status, code: code, message: message}
}

// with attaches structured details to an error payload.
func (e *apiErr) with(details any) *apiErr {
	e.details = details
	return e
}

// detectFormat guesses the inventory format from its first non-space byte, which
// is unambiguous because YAML documents do not normally start with a brace.
func detectFormat(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		return "json"
	}
	return "yaml"
}

func firstHostError(res hosts.Result) string {
	if len(res.Errors) == 0 {
		return "主机清单校验失败"
	}
	first := res.Errors[0]
	if first.Index < 0 {
		return "主机清单校验失败：" + first.Message
	}
	return fmt.Sprintf("第 %d 台主机有问题：%s", first.Index+1, first.Message)
}

// defaultOutputName derives the auto-install image name from the base image.
func defaultOutputName(isoFilename string) string {
	base := strings.TrimSuffix(isoFilename, filepath.Ext(isoFilename))
	return base + "-auto.iso"
}

func pick(v *string, fallback string) string {
	if v == nil {
		return fallback
	}
	return *v
}
