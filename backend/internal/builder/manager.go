// Package builder turns a project into a bootable auto-install ISO: it extracts
// the base image, injects the host inventory into a kickstart, rewrites the boot
// configuration, repacks the ISO with xorriso and finally publishes the artifact.
package builder

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"isobuilder/internal/buildlog"
	"isobuilder/internal/config"
	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// Manager owns the build queue and the running build processes.
type Manager struct {
	cfg    config.Config
	store  *store.Store
	blob   storage.Backend
	logger *slog.Logger

	slots   chan struct{}
	mu      sync.Mutex
	cancels map[string]context.CancelFunc
	logs    map[string]*buildlog.File
	wg      sync.WaitGroup
}

// NewManager builds a manager with the configured concurrency limit.
func NewManager(cfg config.Config, st *store.Store, blob storage.Backend) *Manager {
	return &Manager{
		cfg:     cfg,
		store:   st,
		blob:    blob,
		logger:  slog.Default(),
		slots:   make(chan struct{}, cfg.MaxConcurrentBuilds),
		cancels: map[string]context.CancelFunc{},
		logs:    map[string]*buildlog.File{},
	}
}

// Storage exposes the artifact backend for the HTTP layer.
func (m *Manager) Storage() storage.Backend { return m.blob }

// Enqueue starts a build in the background. The build row must already exist.
func (m *Manager) Enqueue(b model.Build) error {
	logFile, err := buildlog.Create(m.cfg.BuildDir, b.ID)
	if err != nil {
		return fmt.Errorf("create build log: %w", err)
	}
	m.mu.Lock()
	m.logs[b.ID] = logFile
	m.mu.Unlock()

	m.wg.Add(1)
	go func() {
		defer m.wg.Done()

		// Wait for a slot before starting the clock: a build that queues behind
		// others must not spend its timeout budget standing in line.
		m.slots <- struct{}{}
		defer func() { <-m.slots }()

		ctx, cancel := context.WithTimeout(context.Background(), m.cfg.BuildTimeout)
		m.mu.Lock()
		m.cancels[b.ID] = cancel
		m.mu.Unlock()

		// The pipeline marks a successful build itself, because success has
		// extra state (artifact, progress 100). Every other outcome is terminal
		// state that only this wrapper knows about, so it is written here.
		err := m.run(ctx, b, logFile)
		cancel()
		if err != nil {
			m.finishFailed(b.ID, err)
		}
		_ = logFile.Write([]buildlog.Line{{Text: formatOutcome(err, m.cfg.BuildTimeout)}})
		_ = logFile.Close()
		m.forget(b.ID)
	}()
	return nil
}

// finishFailed records the terminal state of a build that did not succeed.
func (m *Manager) finishFailed(buildID string, err error) {
	status, message := model.BuildFailed, err.Error()
	switch {
	case errors.Is(err, context.Canceled):
		status, message = model.BuildCanceled, "已取消"
	case errors.Is(err, context.DeadlineExceeded):
		message = "构建超时（超过 " + m.cfg.BuildTimeout.String() + "）"
	}
	current, getErr := m.store.GetBuild(buildID)
	progress := 0
	if getErr == nil {
		progress = current.Progress
	}
	if finishErr := m.store.FinishBuild(buildID, status, message, progress); finishErr != nil {
		m.logger.Error("记录构建终态失败", "build", buildID, "err", finishErr)
	}
}

func (m *Manager) forget(buildID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.cancels, buildID)
	delete(m.logs, buildID)
}

// Cancel stops a running build. It reports whether the build was active.
func (m *Manager) Cancel(buildID string) bool {
	m.mu.Lock()
	cancel, ok := m.cancels[buildID]
	m.mu.Unlock()
	if ok {
		cancel()
	}
	return ok
}

// LogFile returns the log path and current size of a build, whether or not the
// build is still running.
func (m *Manager) LogFile(buildID string) (string, int64, bool) {
	m.mu.Lock()
	if f, ok := m.logs[buildID]; ok {
		m.mu.Unlock()
		return f.Path(), f.Size(), true
	}
	m.mu.Unlock()
	p := filepath.Join(m.cfg.BuildDir, buildID+".log")
	fi, err := os.Stat(p)
	if err != nil {
		return "", 0, false
	}
	return p, fi.Size(), true
}

// Subscribe streams new log lines of a running build. It returns ok=false when
// the build is not running, in which case the caller replays the file instead.
func (m *Manager) Subscribe(buildID string) (<-chan buildlog.Line, func(), bool) {
	m.mu.Lock()
	f, ok := m.logs[buildID]
	m.mu.Unlock()
	if !ok {
		return nil, func() {}, false
	}
	ch, unsub := f.Subscribe()
	return ch, unsub, true
}

// Wait blocks until every running build finishes. Used for graceful shutdown.
func (m *Manager) Wait() { m.wg.Wait() }

// CancelAll stops every running build, used on shutdown.
func (m *Manager) CancelAll() {
	m.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(m.cancels))
	for _, c := range m.cancels {
		cancels = append(cancels, c)
	}
	m.mu.Unlock()
	for _, c := range cancels {
		c()
	}
}

// Line builds one log line terminated with a newline.
func Line(text string) buildlog.Line {
	return buildlog.Line{Text: text + "\n"}
}

func formatOutcome(err error, timeout time.Duration) string {
	switch {
	case err == nil:
		return "\n=== 构建成功 ===\n"
	case errors.Is(err, context.Canceled):
		return "\n=== 构建已取消 ===\n"
	case errors.Is(err, context.DeadlineExceeded):
		return "\n=== 构建超时（超过 " + timeout.String() + "） ===\n"
	default:
		return "\n=== 构建失败：" + err.Error() + " ===\n"
	}
}

// publisher is the hook the pipeline uses to log and to advance stage state.
type publisher struct {
	mgr     *Manager
	buildID string
	log     *buildlog.File
	stage   string
	index   int
}

func (p *publisher) logf(format string, args ...any) {
	_ = p.log.Write([]buildlog.Line{{Text: fmt.Sprintf(format, args...) + "\n"}})
}

// stage advances the build to a new pipeline stage.
func (p *publisher) stageAt(index int, name string) {
	p.stage = name
	p.index = index
	start, _ := model.StageProgress(index)
	_ = p.mgr.store.SetBuildStage(p.buildID, name, index, start)
	p.logf("\n[%d/%d] %s", index, len(model.Stages), stageLabel(name))
}

// progress reports intra-stage progress, clamped to the stage window.
func (p *publisher) progress(fraction float64) {
	start, end := model.StageProgress(p.index)
	if fraction < 0 {
		fraction = 0
	}
	if fraction > 1 {
		fraction = 1
	}
	value := start + int(float64(end-start)*fraction)
	_ = p.mgr.store.SetBuildStage(p.buildID, p.stage, p.index, value)
}

func stageLabel(name string) string {
	switch name {
	case "mount":
		return "解包原版 ISO"
	case "inject":
		return "注入 kickstart 与主机清单"
	case "boot":
		return "改写引导配置"
	case "build":
		return "xorriso 重建 ISO"
	case "seal":
		return "写入校验并上传对象存储"
	default:
		return name
	}
}

// cleanup removes the scratch directory unless it is being kept for debugging.
func (m *Manager) cleanup(workDir string) {
	if m.cfg.KeepWorkDir {
		return
	}
	if strings.HasPrefix(workDir, filepath.Clean(m.cfg.WorkDir)+string(os.PathSeparator)) {
		_ = os.RemoveAll(workDir)
	}
}
