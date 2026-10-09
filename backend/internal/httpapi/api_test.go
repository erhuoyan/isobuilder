package httpapi

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"sync"
	"testing"

	"isobuilder/internal/buildlog"
	"isobuilder/internal/config"
	"isobuilder/internal/model"
	"isobuilder/internal/storage"
	"isobuilder/internal/store"
)

// stubManager records enqueues without ever running a build, so handler tests
// stay instant and platform independent.
type stubManager struct {
	mu       sync.Mutex
	enqueued []model.Build
	backend  storage.Backend
}

func managerStub(t *testing.T, cfg config.Config, st *store.Store) *stubManager {
	t.Helper()
	return &stubManager{backend: storage.New(cfg.Storage)}
}

func (m *stubManager) Enqueue(b model.Build) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enqueued = append(m.enqueued, b)
	return nil
}

func (m *stubManager) Cancel(string) bool { return true }

func (m *stubManager) LogFile(string) (string, int64, bool) { return "", 0, false }

func (m *stubManager) Subscribe(string) (<-chan buildlog.Line, func(), bool) {
	return nil, func() {}, false
}

func (m *stubManager) Storage() storage.Backend { return m.backend }

func get(t *testing.T, url string) *http.Response {
	t.Helper()
	resp, err := http.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

func postJSON(t *testing.T, url string, body any) *http.Response {
	t.Helper()
	payload, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	resp, err := http.Post(url, "application/json", bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

// postMultipart sends a multipart form with one file field plus extras.
func postMultipart(t *testing.T, url, field, filename, content string, extra map[string]string) *http.Response {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile(field, filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	for k, v := range extra {
		if err := w.WriteField(k, v); err != nil {
			t.Fatalf("write field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	resp, err := http.Post(url, w.FormDataContentType(), &buf)
	if err != nil {
		t.Fatalf("POST %s: %v", url, err)
	}
	return resp
}

func readAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return body
}

// decode unwraps {code, message, data} and re-decodes Data into dst.
func decode(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	raw := readAll(t, resp)
	var env struct {
		Code string          `json:"code"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("decode envelope: %v: %s", err, raw)
	}
	if len(env.Data) == 0 {
		t.Fatalf("envelope has no data: %s", raw)
	}
	if err := json.Unmarshal(env.Data, dst); err != nil {
		t.Fatalf("decode envelope data: %v: %s", err, raw)
	}
}

// failBody reads an envelope error; the API reports failures as HTTP 200 with
// a non-success envelope code, matching the SoybeanAdmin convention.
func failBody(t *testing.T, resp *http.Response) (code, message string, details json.RawMessage) {
	t.Helper()
	raw := readAll(t, resp)
	var env struct {
		Code    string          `json:"code"`
		Message string          `json:"message"`
		Details json.RawMessage `json:"details"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("parse error envelope: %v: %s", err, raw)
	}
	return env.Code, env.Message, env.Details
}
