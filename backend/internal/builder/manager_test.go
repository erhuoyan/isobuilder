package builder

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"isobuilder/internal/config"
	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// newTestManager builds a manager over a throwaway data directory.
func newTestManager(t *testing.T) (*Manager, *store.Store) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", root)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.BuildDir = filepath.Join(root, "builds")
	cfg.WorkDir = filepath.Join(root, "work")
	if err := os.MkdirAll(cfg.BuildDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.WorkDir, 0o755); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(root, "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	m := NewManager(cfg, st, storage.NewSwitcher(storage.New(cfg.Storage)))
	m.logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return m, st
}

// seedBuild inserts a queued build row so the worker has something to run.
func seedBuild(t *testing.T, st *store.Store, id string) model.Build {
	t.Helper()
	b := model.Build{
		ID:         id,
		ProjectID:  "proj-1",
		Status:     model.BuildQueued,
		StageTotal: len(model.Stages),
		Arch:       model.ArchX86,
		OutputName: "out.iso",
		CreatedAt:  store.Now(),
	}
	if err := st.CreateBuild(b); err != nil {
		t.Fatalf("create build: %v", err)
	}
	return b
}

// TestFailedBuildReachesTerminalState guards a real defect found by the
// end-to-end run: when the pipeline returned an error, the build row stayed
// "running" forever, so the UI polled a dead job and the log stream never
// received its terminal event.
func TestFailedBuildReachesTerminalState(t *testing.T) {
	m, st := newTestManager(t)

	// A base image path that does not exist makes the pipeline fail at once.
	b := seedBuild(t, st, "fail-build")
	b.IsoPath = filepath.Join(t.TempDir(), "missing.iso")
	b.KSID = "ks-1"

	if err := st.SetBuildStage(b.ID, "mount", 1, 5); err != nil {
		t.Fatal(err)
	}
	m.finishFailed(b.ID, errors.New("模拟的流水线错误"))

	got, err := st.GetBuild(b.ID)
	if err != nil {
		t.Fatalf("get build: %v", err)
	}
	if got.Status != model.BuildFailed {
		t.Fatalf("status = %q, want %q", got.Status, model.BuildFailed)
	}
	if got.Error == "" {
		t.Fatal("failure reason must be recorded so the operator can see why")
	}
	if got.FinishedAt == "" {
		t.Fatal("finished_at must be set, otherwise the UI cannot show a duration")
	}
	if got.Progress != 5 {
		t.Fatalf("progress = %d, want the stage progress 5 to be preserved", got.Progress)
	}
}

// TestCanceledBuildIsDistinguished proves a cancellation is not reported as a
// failure, which would send the operator hunting for a bug that never happened.
func TestCanceledBuildIsDistinguished(t *testing.T) {
	m, st := newTestManager(t)
	b := seedBuild(t, st, "cancel-build")

	m.finishFailed(b.ID, context.Canceled)

	got, err := st.GetBuild(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.BuildCanceled {
		t.Fatalf("status = %q, want %q", got.Status, model.BuildCanceled)
	}
}

// TestTimeoutMessageExplainsItself keeps the operator-facing wording specific.
func TestTimeoutMessageExplainsItself(t *testing.T) {
	m, st := newTestManager(t)
	m.cfg.BuildTimeout = 90 * time.Minute
	b := seedBuild(t, st, "timeout-build")

	m.finishFailed(b.ID, context.DeadlineExceeded)

	got, err := st.GetBuild(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.BuildFailed {
		t.Fatalf("status = %q, want %q", got.Status, model.BuildFailed)
	}
	if got.Error != "构建超时（超过 1h30m0s）" {
		t.Fatalf("error = %q", got.Error)
	}
}

// TestNeededDiskBytes covers the transient-space estimate: it is the estimate
// cited in the operator's error when a container volume cannot absorb a build.
func TestNeededDiskBytes(t *testing.T) {
	const gib = 1 << 30
	if got := neededDiskBytes(15 * gib); got != 31*gib {
		t.Fatalf("neededDiskBytes(15GiB) = %d, want %d (2x image + 1GiB slack)", got, 31*gib)
	}
	if got := neededDiskBytes(0); got != gib {
		t.Fatalf("neededDiskBytes(0) = %d, want the slack %d", got, gib)
	}
}

// TestCheckDiskSpace exercises the pure decision the guard wraps: an undersized
// volume fails with an operator-facing message; exactly enough space passes.
func TestCheckDiskSpace(t *testing.T) {
	m, _ := newTestManager(t)
	const gib = 1 << 30

	err := checkDiskSpace(5*gib, 11*gib, 9*gib, m.cfg.DataDir)
	if err == nil {
		t.Fatal("undersized volume must be rejected")
	}
	for _, want := range []string{"磁盘空间不足", "5.00 GB", "9.00 GB", "11.00 GB", "对象存储"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error message must help the operator, expected %q in:\n%s", want, err.Error())
		}
	}

	if err := checkDiskSpace(5*gib, 11*gib, 11*gib, m.cfg.DataDir); err != nil {
		t.Fatalf("exactly enough space must pass: %v", err)
	}
}

// TestParseInspectLabel checks the xorriso -pvd_info parsing that replaced the
// isoinfo dependency (isoinfo lives in EPEL on RHEL 9).
func TestParseInspectLabel(t *testing.T) {
	sample := `xorriso 1.5.4 : RockRidge filesystem manipulator, libburnia project.

Drive current: -indev '/data/isos/x.iso'
Volume id    : 'ROCKYFAKE'
PVD address  : 16s
Volume Id    : ROCKYFAKE
Volume Set Id: 
`
	m := volumeIDRe.FindStringSubmatch(sample)
	if m == nil {
		t.Fatal("volume label not extracted from xorriso -pvd_info output")
	}
	if got := trimSpace(m[1]); got != "ROCKYFAKE" {
		t.Fatalf("label = %q, want ROCKYFAKE", got)
	}
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\r') {
		s = s[1:]
	}
	for len(s) > 0 {
		last := s[len(s)-1]
		if last != ' ' && last != '\t' && last != '\r' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}
