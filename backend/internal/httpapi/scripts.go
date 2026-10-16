package httpapi

import (
	"fmt"
	"net/http"
	"strings"

	"isobuilder/internal/model"
	"isobuilder/internal/store"
)

// maxScriptBytes bounds one reusable post-install script (same ceiling as the
// project-level inline script).
const maxScriptBytes = 64 << 10

// handleListScripts returns every script in the library, newest edit first.
func (s *Server) handleListScripts(w http.ResponseWriter, r *http.Request) {
	list, err := s.store.ListScripts()
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "查询脚本失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"scripts": list})
}

// handleCreateScript stores a new script. The body is JSON (not multipart):
// scripts are edited in the browser, there is no file to upload.
func (s *Server) handleCreateScript(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Remark  string `json:"remark"`
		Content string `json:"content"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		fail(w, http.StatusBadRequest, "invalid_name", "脚本名称不能为空")
		return
	}
	// 归一化行尾：粘贴自 Windows/终端的内容可能带 \r\n 或裸 \r，
	// 浏览器 pre-wrap 与 kickstart 注入都按 \n 工作，不归一化会挤成一行。
	req.Content = normalizeScriptNewlines(req.Content)
	if strings.TrimSpace(req.Content) == "" {
		fail(w, http.StatusBadRequest, "empty_script", "脚本内容为空")
		return
	}
	if len(req.Content) > maxScriptBytes {
		fail(w, http.StatusRequestEntityTooLarge, "too_large",
			fmt.Sprintf("脚本超过 %d 字节", maxScriptBytes))
		return
	}

	now := store.Now()
	sc := model.Script{
		ID:        store.NewID(),
		Name:      name,
		Remark:    strings.TrimSpace(req.Remark),
		Content:   req.Content,
		SizeBytes: int64(len(req.Content)),
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.store.CreateScript(sc); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "保存脚本失败: "+err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"script": sc})
}

// handleUpdateScript edits the script. Omitted fields keep their value.
func (s *Server) handleUpdateScript(w http.ResponseWriter, r *http.Request) {
	sc, err := s.store.GetScript(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "脚本")
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

	name := sc.Name
	if req.Name != nil {
		name = strings.TrimSpace(*req.Name)
		if name == "" {
			fail(w, http.StatusBadRequest, "invalid_name", "脚本名称不能为空")
			return
		}
	}
	content := sc.Content
	if req.Content != nil {
		content = normalizeScriptNewlines(*req.Content)
		if strings.TrimSpace(content) == "" {
			fail(w, http.StatusBadRequest, "empty_script", "脚本内容为空")
			return
		}
		if len(content) > maxScriptBytes {
			fail(w, http.StatusRequestEntityTooLarge, "too_large",
				fmt.Sprintf("脚本超过 %d 字节", maxScriptBytes))
			return
		}
	}
	remark := sc.Remark
	if req.Remark != nil {
		remark = strings.TrimSpace(*req.Remark)
	}

	if err := s.store.UpdateScript(sc.ID, name, remark, content); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "保存脚本失败: "+err.Error())
		return
	}
	out, err := s.store.GetScript(sc.ID)
	if err != nil {
		storeFail(w, err, "脚本")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"script": out})
}

// handleDeleteScript removes a script. Projects still bound to it block the
// delete, so a build can never lose the script it references.
func (s *Server) handleDeleteScript(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if _, err := s.store.GetScript(id); err != nil {
		storeFail(w, err, "脚本")
		return
	}
	used, err := s.store.CountProjectsUsingScript(id)
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	if used > 0 {
		fail(w, http.StatusConflict, "script_in_use",
			fmt.Sprintf("还有 %d 个项目绑定该脚本，请先在这些项目里解除绑定", used))
		return
	}
	if err := s.store.DeleteScript(id); err != nil {
		fail(w, http.StatusInternalServerError, "internal", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}
