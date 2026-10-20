package httpapi

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"isobuilder/internal/builder"
	"isobuilder/internal/model"
	"isobuilder/internal/store"
)

// handleCreateBuild queues a build for a project. Everything the worker needs is
// snapshotted into the build row, so editing the project mid-build is safe.
func (s *Server) handleCreateBuild(w http.ResponseWriter, r *http.Request) {
	p, err := s.store.GetProject(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "项目")
		return
	}

	var missing []string
	if p.IsoID == "" {
		missing = append(missing, "iso_id")
	}
	if p.KickstartID == "" {
		missing = append(missing, "kickstart_id")
	}
	if strings.TrimSpace(p.HostsRaw) == "" || p.HostsCount == 0 {
		missing = append(missing, "hosts_raw")
	}
	if len(missing) > 0 {
		fail(w, http.StatusBadRequest, "project_incomplete", "项目配置不完整，请先补齐后再构建",
			map[string]any{"missing": missing})
		return
	}

	iso, err := s.store.GetISO(p.IsoID)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid_iso", "项目绑定的基础镜像已不存在，请重新选择")
		return
	}
	if iso.Status != model.IsoReady {
		fail(w, http.StatusConflict, "iso_not_ready",
			fmt.Sprintf("基础镜像 %s 当前状态为 %s，请等待校验完成", iso.Filename, iso.Status))
		return
	}
	if _, err := os.Stat(iso.Path); err != nil {
		fail(w, http.StatusConflict, "iso_missing", "基础镜像文件不在服务器上: "+iso.Path)
		return
	}
	ks, err := s.store.GetKickstart(p.KickstartID)
	if err != nil {
		fail(w, http.StatusBadRequest, "invalid_kickstart", "项目绑定的 kickstart 模板已不存在，请重新选择")
		return
	}
	if _, err := os.Stat(ks.Path); err != nil {
		fail(w, http.StatusConflict, "kickstart_missing", "kickstart 模板文件不在服务器上: "+ks.Path)
		return
	}

	if abs := builder.MissingTools(p.Arch); len(abs) > 0 {
		fail(w, http.StatusConflict, "missing_tools",
			fmt.Sprintf("构建主机缺少必需工具 %s，无法构建 %s 镜像", strings.Join(abs, "、"), p.Arch))
		return
	}

	// 装机后脚本：脚本库内容在前，项目内联追加在后。在此解析成最终文本，
	// 快照进 build 行 —— 之后改脚本库或改项目都不影响本次构建。
	postScript := p.PostScript
	if p.ScriptID != "" {
		sc, err := s.store.GetScript(p.ScriptID)
		if err != nil {
			fail(w, http.StatusBadRequest, "invalid_script", "项目引用的脚本已不存在，请重新选择")
			return
		}
		postScript = combineScripts(sc.Content, p.PostScript)
	}

	b := model.Build{
		ID:          store.NewID(),
		ProjectID:   p.ID,
		ProjectName: p.Name,
		Status:      model.BuildQueued,
		StageTotal:  len(model.Stages),
		Progress:    0,
		Arch:        p.Arch,
		IsoPath:     iso.Path,
		IsoName:     iso.Name,
		IsoLabel:    p.IsoLabel,
		KSID:        ks.ID,
		HostsJSON:   p.HostsJSON,
		HostsCount:  p.HostsCount,
		OutputName:  p.OutputName,
		PostScript:  postScript,
		PayloadMode: p.PayloadMode,
		CreatedAt:   store.Now(),
	}
	if b.OutputName == "" {
		b.OutputName = defaultOutputName(iso.Filename)
	}
	if err := s.store.CreateBuild(b); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "创建构建任务失败: "+err.Error())
		return
	}
	if err := s.manager.Enqueue(b); err != nil {
		_ = s.store.FinishBuild(b.ID, model.BuildFailed, err.Error(), 0)
		fail(w, http.StatusInternalServerError, "internal", "启动构建失败: "+err.Error())
		return
	}

	out, err := s.store.GetBuild(b.ID)
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"build": out})
}

// handleListBuilds lists builds, optionally scoped to a project.
func (s *Server) handleListBuilds(w http.ResponseWriter, r *http.Request) {
	limit, _ := parseIntQuery(r, "limit")
	list, err := s.store.ListBuilds(r.URL.Query().Get("project_id"), int(limit))
	if err != nil {
		fail(w, http.StatusInternalServerError, "internal", "查询构建失败: "+err.Error())
		return
	}
	// The stored log_bytes column is never written, so without this the list
	// view reported "0 B" for every build, finished ones included. One stat per
	// row is cheap and matches what handleGetBuild already does.
	for i := range list {
		if _, size, ok := s.manager.LogFile(list[i].ID); ok {
			list[i].LogBytes = size
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"builds": list})
}

// handleGetBuild returns one build.
func (s *Server) handleGetBuild(w http.ResponseWriter, r *http.Request) {
	b, err := s.store.GetBuild(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	// The log file is the source of truth for its size in every state; the stored
	// value would otherwise stay at 0 for builds that ran before a restart.
	if _, size, ok := s.manager.LogFile(b.ID); ok {
		b.LogBytes = size
	}
	writeJSON(w, http.StatusOK, map[string]any{"build": b})
}

// handleCancelBuild stops a queued or running build.
func (s *Server) handleCancelBuild(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := s.store.GetBuild(id)
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	if model.Terminal(b.Status) {
		fail(w, http.StatusConflict, "build_finished", "构建已结束，无法取消")
		return
	}
	s.manager.Cancel(id)
	if b.Status == model.BuildQueued {
		// A queued build has no process yet, so terminalise it directly.
		if err := s.store.FinishBuild(id, model.BuildCanceled, "已取消", b.Progress); err != nil {
			fail(w, http.StatusInternalServerError, "internal", err.Error())
			return
		}
	}
	out, err := s.store.GetBuild(id)
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"build": out})
}

// handleRetryBuild re-queues a finished build from its snapshotted inputs.
func (s *Server) handleRetryBuild(w http.ResponseWriter, r *http.Request) {
	src, err := s.store.GetBuild(r.PathValue("id"))
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	if !model.Terminal(src.Status) {
		fail(w, http.StatusConflict, "build_active", "构建仍在进行中，无法重试")
		return
	}
	if _, err := os.Stat(src.IsoPath); err != nil {
		fail(w, http.StatusConflict, "iso_missing", "基础镜像文件已不存在: "+src.IsoPath)
		return
	}
	if abs := builder.MissingTools(src.Arch); len(abs) > 0 {
		fail(w, http.StatusConflict, "missing_tools",
			fmt.Sprintf("构建主机缺少必需工具 %s", strings.Join(abs, "、")))
		return
	}

	b := src
	b.ID = store.NewID()
	b.Status = model.BuildQueued
	b.Stage = ""
	b.StageIndex = 0
	b.Progress = 0
	b.Error = ""
	b.ArtifactID = ""
	b.Artifact = nil
	b.WorkDir = ""
	b.StartedAt = ""
	b.FinishedAt = ""
	b.LogBytes = 0
	b.CreatedAt = store.Now()

	if err := s.store.CreateBuild(b); err != nil {
		fail(w, http.StatusInternalServerError, "internal", "创建构建任务失败: "+err.Error())
		return
	}
	if err := s.manager.Enqueue(b); err != nil {
		_ = s.store.FinishBuild(b.ID, model.BuildFailed, err.Error(), 0)
		fail(w, http.StatusInternalServerError, "internal", "启动构建失败: "+err.Error())
		return
	}
	out, err := s.store.GetBuild(b.ID)
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"build": out})
}

// handleBuildLog streams the build log. SSE is the default; ?format=text returns
// the whole log, which is what a shell user or a screenshot wants.
func (s *Server) handleBuildLog(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	b, err := s.store.GetBuild(id)
	if err != nil {
		storeFail(w, err, "构建")
		return
	}
	path, size, ok := s.manager.LogFile(id)
	if !ok {
		// A wiped log file is not an error: report an empty stream.
		if r.URL.Query().Get("format") == "text" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusOK)
			return
		}
		fail(w, http.StatusNotFound, "log_not_found", "该构建没有日志（日志文件已被清理）")
		return
	}

	offset, err := parseIntQuery(r, "offset")
	if err != nil || offset < 0 {
		offset = 0
	}
	if offset > size {
		offset = 0
	}

	if r.URL.Query().Get("format") == "text" {
		f, err := os.Open(path)
		if err != nil {
			fail(w, http.StatusInternalServerError, "internal", "读取日志失败: "+err.Error())
			return
		}
		defer f.Close()
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			fail(w, http.StatusInternalServerError, "internal", "读取日志失败: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, f)
		return
	}

	s.streamLog(w, r, b, path, offset)
}

// streamLog replays the log from offset and then follows it live until the build
// reaches a terminal state. Each event carries the byte offset of the next line,
// so a dropped connection resumes without duplicating or losing output.
func (s *Server) streamLog(w http.ResponseWriter, r *http.Request, b model.Build, path string, offset int64) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		fail(w, http.StatusInternalServerError, "internal", "当前服务器不支持流式响应")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Accel-Buffering", "no")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()

	send := func(line string, next int64) bool {
		payload, err := json.Marshal(map[string]any{"offset": next, "line": line})
		if err != nil {
			return false
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", payload); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}

	// Subscribe before reading the tail so no line can slip between the two.
	lines, unsub, live := s.manager.Subscribe(b.ID)
	if live {
		defer unsub()
	}

	if !s.replayLog(w, path, offset, send) {
		return
	}

	if !live || model.Terminal(b.Status) {
		s.finishStream(w, b.ID, flusher)
		return
	}

	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case line, open := <-lines:
			if !open {
				s.finishStream(w, b.ID, flusher)
				return
			}
			// A line can be published between the replay and the subscription
			// only in theory; the offset makes it harmless either way.
			if !send(line.Text, line.Offset+int64(len(line.Text))) {
				return
			}
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
				return
			}
			flusher.Flush()
			if b, err := s.store.GetBuild(b.ID); err == nil && model.Terminal(b.Status) {
				s.finishStream(w, b.ID, flusher)
				return
			}
		}
	}
}

// replayLog streams the tail of the log starting at offset.
func (s *Server) replayLog(w io.Writer, path string, offset int64, send func(string, int64) bool) bool {
	f, err := os.Open(path)
	if err != nil {
		return true // the log file may have just been removed
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return true
	}
	reader := newLineReader(f, offset)
	for {
		line, next, err := reader.next()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return true
		}
		if !send(line, next) {
			return false
		}
	}
}

// finishStream sends the terminal event, including the build outcome so the
// client can re-render without another request.
func (s *Server) finishStream(w io.Writer, buildID string, flusher http.Flusher) {
	status := model.BuildSucceeded
	next := int64(0)
	if b, err := s.store.GetBuild(buildID); err == nil {
		status = b.Status
		next = b.LogBytes
		if _, size, ok := s.manager.LogFile(buildID); ok {
			next = size
		}
	}
	payload, _ := json.Marshal(map[string]any{"status": status, "next_offset": next})
	fmt.Fprintf(w, "event: end\ndata: %s\n\n", payload)
	flusher.Flush()
}

// lineReader reads newline terminated chunks from an offset, tracking offsets.
type lineReader struct {
	r      io.Reader
	buf    []byte
	offset int64
}

func newLineReader(r io.Reader, offset int64) *lineReader {
	return &lineReader{r: r, offset: offset}
}

func (l *lineReader) next() (string, int64, error) {
	for {
		if idx := bytes.IndexByte(l.buf, '\n'); idx >= 0 {
			line := string(l.buf[:idx+1])
			l.buf = l.buf[idx+1:]
			l.offset += int64(len(line))
			return line, l.offset, nil
		}
		chunk := make([]byte, 64<<10)
		n, err := l.r.Read(chunk)
		if n > 0 {
			l.buf = append(l.buf, chunk[:n]...)
			continue
		}
		if err == io.EOF {
			if len(l.buf) > 0 {
				line := string(l.buf)
				l.buf = nil
				l.offset += int64(len(line))
				return line, l.offset, nil
			}
			return "", l.offset, io.EOF
		}
		if err != nil {
			return "", l.offset, err
		}
	}
}

// combineScripts renders the library script followed by the project's inline
// script: the shared baseline runs first, project-specific steps append after.
func combineScripts(library, inline string) string {
	library = strings.TrimRight(library, "\n")
	inline = strings.TrimSpace(inline)
	switch {
	case library == "":
		return inline
	case inline == "":
		return library
	default:
		return library + "\n\n" + inline + "\n"
	}
}
