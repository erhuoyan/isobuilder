package store

import (
	"path/filepath"
	"testing"

	"isobuilder/internal/model"
)

func isoWithPath(id, path string) model.ISO {
	return model.ISO{ID: id, Path: path, Status: model.IsoReady, CreatedAt: Now()}
}

// Managed ISOs are laid out as isos/<id>/<filename>: the id keeps names
// unique, the original filename stays readable.
func TestManagedISORelPath(t *testing.T) {
	got := ManagedISORelPath("07bcb70d73f7", "openEuler-22.03-LTS-SP4-aarch64-dvd.iso")
	if want := "isos/07bcb70d73f7/openEuler-22.03-LTS-SP4-aarch64-dvd.iso"; got != want {
		t.Fatalf("ManagedISORelPath = %q, want %q", got, want)
	}
	// A path-like filename must be reduced to its base so it cannot escape.
	got = ManagedISORelPath("abc", "../../etc/passwd")
	if want := "isos/abc/passwd"; got != want {
		t.Fatalf("ManagedISORelPath traversal = %q, want %q", got, want)
	}
}

// Paths are stored relative to data_dir, so moving data_dir never orphans a row.
func TestISOPathResolvesUnderCurrentDataDir(t *testing.T) {
	rel := ManagedISORelPath("abc", "x.iso")
	for _, dir := range []string{"/var/lib/isobuilder", "/opt/isobuilder/data", "/srv/data"} {
		got := ISOPath(dir, isoWithPath("abc", rel))
		if want := filepath.Join(dir, "isos", "abc", "x.iso"); got != want {
			t.Errorf("ISOPath(%s) = %q, want %q", dir, got, want)
		}
	}
	// Absolute rows (not yet migrated) pass through unchanged.
	if got := ISOPath("/var/lib/isobuilder", isoWithPath("abc", "/elsewhere/x.iso")); got != "/elsewhere/x.iso" {
		t.Errorf("absolute path = %q, want passthrough", got)
	}
}

func TestRelISOPathRejectsOutside(t *testing.T) {
	if _, err := RelISOPath("/var/lib/isobuilder", "/etc/passwd"); err == nil {
		t.Fatal("expected error for a path outside data_dir")
	}
	if _, err := RelISOPath("/var/lib/isobuilder", "/var/lib/isobuilder/../x"); err == nil {
		t.Fatal("expected error for a path escaping via ..")
	}
}

// The startup migration rewrites absolute rows to relative ones and is
// idempotent.
func TestNormalizeISOPaths(t *testing.T) {
	dataDir := t.TempDir()
	st := OpenTest(t, filepath.Join(dataDir, "test.db"))

	for _, s := range []struct{ id, path string }{
		{"old1", dataDir + "/isos/old1-a.iso"},
		{"old2", dataDir + "/isos/b.iso"},
		{"already", "isos/c.iso"},
	} {
		if err := st.CreateISO(isoWithPath(s.id, s.path)); err != nil {
			t.Fatalf("seed %s: %v", s.id, err)
		}
	}

	n, err := st.NormalizeISOPaths(dataDir)
	if err != nil {
		t.Fatalf("NormalizeISOPaths: %v", err)
	}
	if n != 2 {
		t.Fatalf("converted %d rows, want 2 (already-relative rows are skipped)", n)
	}
	for id, want := range map[string]string{
		"old1":    "isos/old1-a.iso",
		"old2":    "isos/b.iso",
		"already": "isos/c.iso",
	} {
		iso, err := st.GetISO(id)
		if err != nil {
			t.Fatalf("GetISO %s: %v", id, err)
		}
		if iso.Path != want {
			t.Errorf("%s path = %q, want %q", id, iso.Path, want)
		}
	}
	if n, err := st.NormalizeISOPaths(dataDir); err != nil || n != 0 {
		t.Fatalf("second run: n=%d err=%v, want 0/nil", n, err)
	}
}
