package httpapi

import (
	"net/http"
	"strings"
	"testing"

	"isobuilder/internal/model"
)

// deleteReq sends a DELETE request.
func deleteReq(t *testing.T, url string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodDelete, url, nil)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", url, err)
	}
	return resp
}

// TestScriptLibraryCRUD pins the script library lifecycle: create → list →
// update → delete, with the body surviving each round trip.
func TestScriptLibraryCRUD(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	body := "#!/bin/bash\necho install-agent"

	resp := postJSON(t, ts.URL+"/api/scripts", map[string]any{
		"name": "装监控 agent", "remark": "所有机器基线", "content": body,
	})
	var created struct {
		Script model.Script `json:"script"`
	}
	decode(t, resp, &created)
	if created.Script.ID == "" {
		t.Fatal("创建脚本未返回 id")
	}
	if created.Script.SizeBytes != int64(len(body)) {
		t.Fatalf("size_bytes = %d, want %d", created.Script.SizeBytes, len(body))
	}

	resp = get(t, ts.URL+"/api/scripts")
	var list struct {
		Scripts []model.Script `json:"scripts"`
	}
	decode(t, resp, &list)
	if len(list.Scripts) != 1 || list.Scripts[0].Content != body {
		t.Fatalf("列表内容不符: %+v", list.Scripts)
	}

	resp = putJSON(t, ts.URL+"/api/scripts/"+created.Script.ID, map[string]any{
		"content": "#!/bin/bash\necho v2",
	})
	var updated struct {
		Script model.Script `json:"script"`
	}
	decode(t, resp, &updated)
	if !strings.Contains(updated.Script.Content, "v2") {
		t.Fatalf("更新后的内容不对: %q", updated.Script.Content)
	}
	if updated.Script.Name != "装监控 agent" {
		t.Fatalf("未传 name 时应保持不变, 得到 %q", updated.Script.Name)
	}

	resp = deleteReq(t, ts.URL+"/api/scripts/"+created.Script.ID)
	var empty struct{}
	decode(t, resp, &empty)

	resp = get(t, ts.URL+"/api/scripts")
	decode(t, resp, &list)
	if len(list.Scripts) != 0 {
		t.Fatalf("删除后仍剩 %d 条", len(list.Scripts))
	}
}

// TestScriptValidation guards the input bounds: an empty body and an oversized
// body must be rejected at the API, not baked into an ISO later.
func TestScriptValidation(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	resp := postJSON(t, ts.URL+"/api/scripts", map[string]any{"name": "x", "content": "   "})
	if code, _, _ := failBody(t, resp); code != "empty_script" {
		t.Fatalf("空内容应报 empty_script, 得到 %q", code)
	}

	resp = postJSON(t, ts.URL+"/api/scripts", map[string]any{"name": "  ", "content": "echo x"})
	if code, _, _ := failBody(t, resp); code != "invalid_name" {
		t.Fatalf("空名称应报 invalid_name, 得到 %q", code)
	}

	big := strings.Repeat("x", maxScriptBytes+1)
	resp = postJSON(t, ts.URL+"/api/scripts", map[string]any{"name": "big", "content": big})
	if code, _, _ := failBody(t, resp); code != "too_large" {
		t.Fatalf("超长应报 too_large, 得到 %q", code)
	}
}

// TestScriptNewlineNormalization guards against the "one long line" bug: text
// pasted from Windows editors or some terminals carries \r\n or bare \r, which
// browsers and the kickstart base64 injection treat as non-newline. Every
// script entry point must normalize to \n before persisting.
func TestScriptNewlineNormalization(t *testing.T) {
	ts, _ := newTestServer(t)
	defer ts.Close()

	// CRLF on create; bare CR and CRLF mixed on update.
	resp := postJSON(t, ts.URL+"/api/scripts", map[string]any{
		"name":    "crlf",
		"content": "#!/bin/bash\r\necho one\r\necho two\r\n",
	})
	var created struct {
		Script model.Script `json:"script"`
	}
	decode(t, resp, &created)
	want := "#!/bin/bash\necho one\necho two\n"
	if created.Script.Content != want {
		t.Fatalf("create: CRLF not normalized:\n got %q\nwant %q", created.Script.Content, want)
	}

	resp = putJSON(t, ts.URL+"/api/scripts/"+created.Script.ID, map[string]any{
		"content": "echo a\recho b\r\necho c",
	})
	var updated struct {
		Script model.Script `json:"script"`
	}
	decode(t, resp, &updated)
	want = "echo a\necho b\necho c"
	if updated.Script.Content != want {
		t.Fatalf("update: CR not normalized:\n got %q\nwant %q", updated.Script.Content, want)
	}
}

// TestScriptDeleteBlockedWhenBound proves a script still referenced by a
// project cannot be deleted: otherwise a later build would lose its script.
func TestScriptDeleteBlockedWhenBound(t *testing.T) {
	ts, isoDir := newTestServer(t)
	defer ts.Close()
	isoID := seedISO(t, ts, isoDir)
	ksID := seedKickstart(t, ts)

	resp := postJSON(t, ts.URL+"/api/scripts", map[string]any{
		"name": "基线脚本", "content": "#!/bin/bash\ntrue",
	})
	var created struct {
		Script model.Script `json:"script"`
	}
	decode(t, resp, &created)

	// 建一个绑定该脚本的项目
	resp = postJSON(t, ts.URL+"/api/projects", map[string]any{
		"name": "ps-script", "arch": "x86_64", "iso_id": isoID, "kickstart_id": ksID,
		"hosts_raw": validHosts, "script_id": created.Script.ID,
	})
	var proj struct {
		Project model.Project `json:"project"`
	}
	decode(t, resp, &proj)
	if proj.Project.ScriptID != created.Script.ID {
		t.Fatalf("项目未绑定脚本: %q", proj.Project.ScriptID)
	}
	if proj.Project.Script == nil || proj.Project.Script.Name != "基线脚本" {
		t.Fatalf("项目详情应内联脚本元信息, 得到 %+v", proj.Project.Script)
	}
	if proj.Project.Script.Content != "" {
		t.Fatal("内联的脚本不应携带正文（响应膨胀）")
	}

	// 有引用 → 删除被拒
	resp = deleteReq(t, ts.URL+"/api/scripts/"+created.Script.ID)
	if code, _, _ := failBody(t, resp); code != "script_in_use" {
		t.Fatalf("被引用时应报 script_in_use, 得到 %q", code)
	}

	// 解绑后可删
	resp = putJSON(t, ts.URL+"/api/projects/"+proj.Project.ID, map[string]any{
		"hosts_raw": validHosts, "script_id": "",
	})
	var unbound struct {
		Project model.Project `json:"project"`
	}
	decode(t, resp, &unbound)
	if unbound.Project.ScriptID != "" {
		t.Fatalf("解绑失败: %q", unbound.Project.ScriptID)
	}

	resp = deleteReq(t, ts.URL+"/api/scripts/"+created.Script.ID)
	var empty struct{}
	decode(t, resp, &empty)
}

// TestCombineScripts pins the injection order: library baseline first, then the
// project-specific inline script.
func TestCombineScripts(t *testing.T) {
	cases := []struct {
		name         string
		library      string
		inline       string
		wantContains []string
		wantFirst    string
	}{
		{"both", "#!/bin/bash\necho base", "echo extra", []string{"echo base", "echo extra"}, "#!/bin/bash"},
		{"library only", "echo base", "", []string{"echo base"}, "echo base"},
		{"inline only", "", "echo extra", []string{"echo extra"}, "echo extra"},
		{"neither", "", "", nil, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := combineScripts(tc.library, tc.inline)
			for _, frag := range tc.wantContains {
				if !strings.Contains(got, frag) {
					t.Fatalf("结果缺少 %q: %q", frag, got)
				}
			}
			if tc.wantFirst != "" && !strings.HasPrefix(got, tc.wantFirst) {
				t.Fatalf("库脚本应排在前, 得到 %q", got)
			}
			if tc.library != "" && tc.inline != "" {
				if strings.Index(got, "echo base") > strings.Index(got, "echo extra") {
					t.Fatal("库脚本必须在内联脚本之前")
				}
			}
		})
	}
}
