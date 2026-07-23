package runtime_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestCtxHTTPGetURLMode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer upstream.Close()

	rt := runtime.New(runtime.Dependencies{
		HTTP: httpconn.New(nil, nil),
	})
	script := `
function execute(ctx) {
  var res = ctx.http.get({ url: "` + upstream.URL + `/health", timeout: "5s" });
  return { status_code: res.status_code, body: res.body, ok: res.status_code === 200 };
}
`
	out, err := rt.Execute(context.Background(), script, map[string]interface{}{}, 5*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	m, ok := out.(map[string]interface{})
	if !ok {
		t.Fatalf("out=%T", out)
	}
	if m["ok"] != true {
		t.Fatalf("out=%v", m)
	}
	body, _ := m["body"].(map[string]interface{})
	if body["status"] != "ok" {
		t.Fatalf("body=%v", body)
	}
}

func TestCtxHTTPRequestRequiresMethod(t *testing.T) {
	rt := runtime.New(runtime.Dependencies{
		HTTP: httpconn.New(nil, nil),
	})
	script := `
function execute(ctx) {
  return ctx.http.request({ url: "http://127.0.0.1:9/x" });
}
`
	_, err := rt.Execute(context.Background(), script, nil, 2*time.Second)
	if err == nil {
		t.Fatal("expected error")
	}
}
