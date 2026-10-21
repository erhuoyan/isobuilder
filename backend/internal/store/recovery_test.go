package store

import (
	"path/filepath"
	"testing"

	"isobuilder/internal/model"
)

// An image left mid-transfer by a dead process must not stay in a
// non-terminal state: the goroutine that would finish it died with the
// previous process, and a row stuck in "downloading" looks like work in
// progress forever. Reopening the store marks it failed instead.
func TestReopenFailsInterruptedISOs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "isobuilder.db")

	st := OpenTest(t, path)
	for _, iso := range []model.ISO{
		{ID: "down00000001", Filename: "a.iso", Status: model.IsoDownloading, Source: model.SourceURL, CreatedAt: Now()},
		{ID: "hash00000002", Filename: "b.iso", Status: model.IsoHashing, Source: model.SourceURL, CreatedAt: Now()},
		{ID: "ready0000003", Filename: "c.iso", Status: model.IsoReady, Source: model.SourceRegistered, CreatedAt: Now()},
	} {
		if err := st.CreateISO(iso); err != nil {
			t.Fatalf("create iso %s: %v", iso.ID, err)
		}
	}

	// A second open of the same file is what a restart looks like.
	st2 := OpenTest(t, path)

	for _, id := range []string{"down00000001", "hash00000002"} {
		iso, err := st2.GetISO(id)
		if err != nil {
			t.Fatalf("get iso %s: %v", id, err)
		}
		if iso.Status != model.IsoError {
			t.Errorf("iso %s status = %q, want %q", id, iso.Status, model.IsoError)
		}
		if iso.Error == "" {
			t.Errorf("iso %s has no error message explaining the interruption", id)
		}
	}

	ready, err := st2.GetISO("ready0000003")
	if err != nil {
		t.Fatalf("get ready iso: %v", err)
	}
	if ready.Status != model.IsoReady {
		t.Errorf("a finished image must be left alone, got %q", ready.Status)
	}
}
