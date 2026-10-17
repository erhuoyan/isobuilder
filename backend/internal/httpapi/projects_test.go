package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"isobuilder/internal/config"
	"isobuilder/internal/hosts"
	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// newTestServer builds an API server over a throwaway data directory.
func newTestServer(t *testing.T) (*httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	t.Setenv("ISOBUILDER_DATA_DIR", root)
	cfg, err := config.Load("")
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	cfg.DBPath = root + "/test.db"
	cfg.WebDir = root + "/web"
	cfg.BuiltinDir = root + "/builtin"
	if err := os.MkdirAll(cfg.WebDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(cfg.BuiltinDir, 0o755); err != nil {
		t.Fatal(err)
	}

	st := store.OpenTest(t, cfg.DBPath)

	blob := storage.NewSwitcher(storage.New(cfg.Storage))
	api := New(cfg, st, managerStub(t, cfg, st), blob, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return httptest.NewServer(api.Handler()), cfg.IsoDir
}

const validHosts = `hosts:
- sn: "SN0001"
  hostname: node-01
  root_password: test
  disk_target_size: 480
  bmc_ip: 10.0.0.1
  network:
    ip: 172.16.0.11
    netmask: 255.255.255.0
    gateway: 172.16.0.1
    mac: "00:11:22:33:44:55"
`

// seedISO creates a ready ISO record on disk.
func seedISO(t *testing.T, ts *httptest.Server, isoDir string) string {
	t.Helper()
	path := isoDir + "/fake.iso"
	if err := os.WriteFile(path, bytes.Repeat([]byte{1}, 1<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	resp := postJSON(t, ts.URL+"/api/isos/register", map[string]any{"path": path, "arch": "x86_64"})
	var body struct {
		Iso model.ISO `json:"iso"`
	}
	decode(t, resp, &body)
	return body.Iso.ID
}

func seedKickstart(t *testing.T, ts *httptest.Server) string {
	t.Helper()
	resp := postMultipart(t, ts.URL+"/api/kickstarts", "file", "demo.ks", "%%HOSTS_JSON_PLACEHOLDER%%", map[string]string{"arch": "x86_64", "name": "demo.ks"})
	var body struct {
		Kickstart model.Kickstart `json:"kickstart"`
	}
	decode(t, resp, &body)
	return body.Kickstart.ID
}

// TestCreateProjectPersistsHosts proves the raw inventory survives the
// create → store → read round trip. It guards the commit that previously lost
// hosts_raw while keeping hosts_count.
func TestCreateProjectPersistsHosts(t *testing.T) {
	ts, isoDir := newTestServer(t)
	defer ts.Close()
	isoID := seedISO(t, ts, isoDir)
	ksID := seedKickstart(t, ts)

	resp := postJSON(t, ts.URL+"/api/projects", map[string]any{
		"name": "p1", "arch": "x86_64", "iso_id": isoID, "kickstart_id": ksID,
		"hosts_raw": validHosts, "hosts_format": "auto",
	})
	var created struct {
		Project model.Project `json:"project"`
	}
	decode(t, resp, &created)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("create status %d", resp.StatusCode)
	}
	if created.Project.HostsRaw != validHosts {
		t.Fatalf("hosts_raw lost in create response (len %d)", len(created.Project.HostsRaw))
	}
	if created.Project.HostsCount != 1 {
		t.Fatalf("hosts_count = %d, want 1", created.Project.HostsCount)
	}

	resp = get(t, ts.URL+"/api/projects/"+created.Project.ID)
	var fetched struct {
		Project model.Project `json:"project"`
	}
	decode(t, resp, &fetched)
	if fetched.Project.HostsRaw != validHosts {
		t.Fatalf("hosts_raw lost after fetching (len %d)", len(fetched.Project.HostsRaw))
	}

	// Build readiness must not flag hosts_raw as missing.
	resp = postJSON(t, ts.URL+"/api/projects/"+created.Project.ID+"/builds", nil)
	code, _, details := failBody(t, resp)
	if code == "project_incomplete" {
		var missing []string
		var detail struct {
			Missing []string `json:"missing"`
		}
		_ = json.Unmarshal(details, &detail)
		missing = detail.Missing
		for _, m := range missing {
			if m == "hosts_raw" {
				t.Fatal("build precheck wrongly flags hosts_raw as missing")
			}
		}
	}
}

// TestProjectListOmitsRawInventory confirms list responses stay shallow.
func TestProjectListOmitsRawInventory(t *testing.T) {
	ts, isoDir := newTestServer(t)
	defer ts.Close()
	isoID := seedISO(t, ts, isoDir)
	ksID := seedKickstart(t, ts)
	postJSON(t, ts.URL+"/api/projects", map[string]any{
		"name": "p2", "arch": "x86_64", "iso_id": isoID, "kickstart_id": ksID,
		"hosts_raw": validHosts,
	})

	resp := get(t, ts.URL+"/api/projects")
	body := readAll(t, resp)
	// The raw inventory contains the root password; the summary does not. Its
	// presence in the list response proves the raw body leaked.
	if strings.Contains(string(body), "root_password") {
		t.Fatal("project list must not embed the raw inventory")
	}
	if !strings.Contains(string(body), `"hosts_raw":""`) && strings.Contains(string(body), `"hosts_raw":"SN`) {
		t.Fatal("project list embeds the raw inventory body")
	}
}

func TestValidateHosts(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	cases := []struct {
		name    string
		body    string
		wantOK  bool
		codeRef map[string]bool // error fragments expected in messages
	}{
		{"valid single", validHosts, true, nil},
		{"empty", "", false, map[string]bool{"为空": true}},
		{
			"gateway mismatch", `hosts:
- sn: S1
  hostname: h1
  network: {ip: 172.16.0.11, netmask: 255.255.255.0, gateway: 10.0.0.1, mac: "00:11:22:33:44:55"}
`, false, map[string]bool{"不在同一网段": true},
		},
		{
			"duplicate sn", `hosts:
- sn: S1
  hostname: h1
  network: {ip: 1.1.1.1, netmask: 255.255.255.0, mac: "00:11:22:33:44:55"}
- sn: S1
  hostname: h2
  network: {ip: 1.1.1.2, netmask: 255.255.255.0, mac: "00:11:22:33:44:56"}
`, false, map[string]bool{"重复": true},
		},
		{
			"odd bond slaves", `hosts:
- sn: S1
  hostname: h1
  network: {ip: 1.1.1.1, netmask: 255.255.255.0, bond: {mode: 4, slaves: ["00:11:22:33:44:55", "00:11:22:33:44:56", "00:11:22:33:44:57"]}}
`, false, map[string]bool{"偶数个": true},
		},
		{
			"numeric sn accepted", `hosts:
- sn: 219344339114
  hostname: h1
  network: {ip: 1.1.1.1, netmask: 255.255.255.0, mac: "00:11:22:33:44:55"}
`, true, nil,
		},
		{
			"prefix canonicalised", `hosts:
- sn: S1
  hostname: h1
  network: {ip: 1.1.1.1, prefix: 27, mac: "00:11:22:33:44:55"}
`, true, nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := postJSON(t, ts.URL+"/api/validate/hosts", map[string]any{"content": tc.body, "format": "auto"})
			var res hosts.Result
			decode(t, resp, &res)
			if res.OK != tc.wantOK {
				t.Fatalf("ok = %v, want %v; errors=%v", res.OK, tc.wantOK, res.Errors)
			}
			for frag := range tc.codeRef {
				found := false
				for _, e := range res.Errors {
					if strings.Contains(e.Message, frag) {
						found = true
					}
				}
				if !found {
					t.Fatalf("expected error containing %q, got %v", frag, res.Errors)
				}
			}
		})
	}
}

// TestValidateHostsSummaryFields pins the summary contract the admin SPA renders:
// each row must carry the gateway and disk size the operator configured, with
// empty/zero meaning "not configured". The wizard swapped its BMC column for
// these, so a regression silently breaks the preview tables.
func TestValidateHostsSummaryFields(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/validate/hosts", map[string]any{"content": validHosts, "format": "auto"})
	var res hosts.Result
	decode(t, resp, &res)
	if !res.OK || len(res.Summary) != 1 {
		t.Fatalf("expected one valid host, got ok=%v summary=%v errors=%v", res.OK, res.Summary, res.Errors)
	}
	row := res.Summary[0]
	if row.Gateway != "172.16.0.1" {
		t.Fatalf("gateway = %q, want 172.16.0.1", row.Gateway)
	}
	if row.DiskGB != 480 {
		t.Fatalf("disk_gb = %d, want 480", row.DiskGB)
	}

	resp = postJSON(t, ts.URL+"/api/validate/hosts", map[string]any{"content": `hosts:
- sn: S1
  hostname: h1
  network: {ip: 1.1.1.1, netmask: 255.255.255.0, mac: "00:11:22:33:44:55"}
`, "format": "auto"})
	decode(t, resp, &res)
	if !res.OK || len(res.Summary) != 1 {
		t.Fatalf("expected one valid host, got ok=%v errors=%v", res.OK, res.Errors)
	}
	if row := res.Summary[0]; row.Gateway != "" || row.DiskGB != 0 {
		t.Fatalf("unconfigured host must report empty gateway and zero disk_gb, got %q/%d", row.Gateway, row.DiskGB)
	}
}

// TestRegisterUnreadableFileFailsFast guards the operator trap of copying an ISO
// in as root: the file is unreadable for the service account, and the failure
// must be reported at registration with the actual fix, not minutes later as a
// checksum error.
func TestRegisterUnreadableFileFailsFast(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, where every file is readable")
	}
	ts, isoDir := newTestServer(t)
	defer ts.Close()

	path := filepath.Join(isoDir, "root-owned.iso")
	if err := os.WriteFile(path, bytes.Repeat([]byte{7}, 4096), 0o600); err != nil {
		t.Fatal(err)
	}
	// Drop read permission for everyone including the owner running this test.
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })

	resp := postJSON(t, ts.URL+"/api/isos/register", map[string]any{
		"source": "path", "path": path, "arch": "x86_64",
	})
	code, message, _ := failBody(t, resp)

	// Failures surface as HTTP 200 with a biz code (SoybeanAdmin envelope), so
	// the check is on the code, not the status.
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (failures are flattened), with a biz code", resp.StatusCode)
	}
	if code != "path_unreadable" {
		t.Fatalf("code = %q, want path_unreadable", code)
	}
	if !strings.Contains(message, "chown") {
		t.Fatalf("message must name the fix, got %q", message)
	}

	// A rejected registration must not leave a broken record behind.
	var listed struct {
		ISOs []struct {
			Status string `json:"status"`
		} `json:"isos"`
	}
	decode(t, get(t, ts.URL+"/api/isos"), &listed)
	if len(listed.ISOs) != 0 {
		t.Fatalf("rejected registration left %d record(s)", len(listed.ISOs))
	}
}

// TestPresignFallback covers the local-storage artifact download path.
func TestArtifactLocalDownload(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()
	// storage disabled → artifact download would stream local file; here we only
	// prove a missing artifact fails cleanly via the envelope code.
	resp := get(t, ts.URL+"/api/artifacts/no-such-id/download")
	code, _, _ := failBody(t, resp)
	if code != "not_found" {
		t.Fatalf("code = %q, want not_found", code)
	}
}

// TestProjectPostScriptRoundTrip pins the create → read → update lifecycle of
// the project-level post_script: it must survive persistence, an absent field
// in a later update must not wipe it, and an explicit "" must.
func TestProjectPostScriptRoundTrip(t *testing.T) {
	ts, isoDir := newTestServer(t)
	defer ts.Close()
	isoID := seedISO(t, ts, isoDir)
	ksID := seedKickstart(t, ts)

	script := "#!/bin/bash\nyum install -y htop\necho done"

	// 1. create with a script
	resp := postJSON(t, ts.URL+"/api/projects", map[string]any{
		"name": "ps1", "arch": "x86_64", "iso_id": isoID, "kickstart_id": ksID,
		"hosts_raw": validHosts, "hosts_format": "auto", "post_script": script,
	})
	var created struct {
		Project model.Project `json:"project"`
	}
	decode(t, resp, &created)
	if created.Project.PostScript != script {
		t.Fatalf("post_script lost in create: got %q", created.Project.PostScript)
	}

	// 2. fetch back
	resp = get(t, ts.URL+"/api/projects/"+created.Project.ID)
	var fetched struct {
		Project model.Project `json:"project"`
	}
	decode(t, resp, &fetched)
	if fetched.Project.PostScript != script {
		t.Fatalf("post_script lost after fetch: got %q", fetched.Project.PostScript)
	}

	// 3. update (PUT) without post_script → unchanged
	resp = putJSON(t, ts.URL+"/api/projects/"+created.Project.ID, map[string]any{
		"name": "ps1-renamed", "hosts_raw": validHosts,
	})
	decode(t, resp, &fetched)
	if fetched.Project.PostScript != script {
		t.Fatalf("post_script must survive an update that omits it: got %q", fetched.Project.PostScript)
	}

	// 4. update (PUT) with "" → cleared
	resp = putJSON(t, ts.URL+"/api/projects/"+created.Project.ID, map[string]any{
		"name": "ps1-cleared", "hosts_raw": validHosts, "post_script": "",
	})
	decode(t, resp, &fetched)
	if fetched.Project.PostScript != "" {
		t.Fatalf("post_script must be cleared by an explicit empty string: got %q", fetched.Project.PostScript)
	}
}

// TestProjectPostScriptTooLarge guards the size bound: a script bigger than
// maxPostScriptBytes must be rejected at validation, not baked into the ISO.
func TestProjectPostScriptTooLarge(t *testing.T) {
	ts, isoDir := newTestServer(t)
	defer ts.Close()
	isoID := seedISO(t, ts, isoDir)
	ksID := seedKickstart(t, ts)

	big := strings.Repeat("x", maxPostScriptBytes+1)
	resp := postJSON(t, ts.URL+"/api/projects", map[string]any{
		"name": "ps-big", "arch": "x86_64", "iso_id": isoID, "kickstart_id": ksID,
		"hosts_raw": validHosts, "post_script": big,
	})
	code, _, _ := failBody(t, resp)
	if code != "invalid_post_script" {
		t.Fatalf("code = %q, want invalid_post_script", code)
	}
}
