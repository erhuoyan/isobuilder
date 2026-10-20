package httpapi

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"testing"

	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// The build list is what the UI renders, and its log column reads log_bytes.
// That column is never written to the database, so the handler has to fill it
// from the log file — otherwise every row shows "0 B", finished builds
// included. This pins that behaviour.
func TestListBuildsFillsLogBytes(t *testing.T) {
	root := t.TempDir()
	cfg, err := testConfig(t, root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.DBPath = root + "/test.db"
	cfg.WebDir = root + "/web"
	cfg.BuiltinDir = root + "/builtin"
	if err := os.MkdirAll(cfg.WebDir, 0o755); err != nil {
		t.Fatal(err)
	}

	st := store.OpenTest(t, cfg.DBPath)
	mgr := &stubManager{
		backend:  storage.New(cfg.Storage),
		logSizes: map[string]int64{"build-with-log": 2048},
	}
	api := New(cfg, st, mgr, storage.NewSwitcher(storage.New(cfg.Storage)),
		slog.New(slog.NewTextHandler(io.Discard, nil)))
	ts := httptest.NewServer(api.Handler())
	defer ts.Close()

	for _, b := range []model.Build{
		{ID: "build-with-log", ProjectID: "p1", ProjectName: "demo",
			Status: model.BuildSucceeded, CreatedAt: store.Now()},
		{ID: "build-no-log", ProjectID: "p1", ProjectName: "demo",
			Status: model.BuildFailed, CreatedAt: store.Now()},
	} {
		if err := st.CreateBuild(b); err != nil {
			t.Fatalf("seed build %s: %v", b.ID, err)
		}
	}

	resp := get(t, ts.URL+"/api/builds")
	var body struct {
		Builds []model.Build `json:"builds"`
	}
	decode(t, resp, &body)

	got := make(map[string]int64, len(body.Builds))
	for _, b := range body.Builds {
		got[b.ID] = b.LogBytes
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 builds, got %d: %v", len(got), got)
	}
	if got["build-with-log"] != 2048 {
		t.Errorf("log_bytes = %d, want 2048", got["build-with-log"])
	}
	if got["build-no-log"] != 0 {
		t.Errorf("log_bytes = %d, want 0 when no log file exists", got["build-no-log"])
	}
}
