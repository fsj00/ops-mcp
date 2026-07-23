package http_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/fsj00/ops-mcp/internal/config"
	httpconn "github.com/fsj00/ops-mcp/internal/connector/http"
)

func TestHTTPConnectorDo(t *testing.T) {
	dir := t.TempDir()
	var sawAuth string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization")
		if r.Header.Get("X-Trace") != "1" {
			http.Error(w, "missing trace", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"ok": "yes"})
	}))
	defer upstream.Close()

	hosts := filepath.Join(dir, "hosts.yaml")
	apis := filepath.Join(dir, "apis.yaml")
	cfgPath := filepath.Join(dir, "ops-mcp.yaml")
	oa := filepath.Join(dir, "oa.yaml")
	_ = os.WriteFile(hosts, []byte("hosts: []\n"), 0o600)
	_ = os.WriteFile(oa, []byte("openapi: 3.0.0\npaths: {}\n"), 0o600)
	_ = os.WriteFile(apis, []byte(`
apis:
  - name: demo
    openapi:
      path: "`+oa+`"
    endpoint:
      base_url: "`+upstream.URL+`"
      timeout: 3s
    headers:
      Authorization: "Bearer from-config"
`), 0o600)
	_ = os.WriteFile(cfgPath, []byte(`
server:
  port: 1
plugins:
  dir: "`+dir+`"
config:
  hosts: "`+hosts+`"
  apis: "`+apis+`"
`), 0o600)

	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	c := httpconn.New(cfg, nil)
	res, err := c.Do(context.Background(), httpconn.Request{
		API:    "demo",
		Method: "GET",
		Path:   "/ping",
		Headers: map[string]string{
			"Authorization": "Bearer override",
			"X-Trace":       "1",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if sawAuth != "Bearer override" {
		t.Fatalf("auth=%q want override", sawAuth)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	body, _ := res.Body.(map[string]interface{})
	if body["ok"] != "yes" {
		t.Fatalf("body=%v", res.Body)
	}
}

func TestHTTPConnectorURLMode(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer upstream.Close()

	c := httpconn.New(nil, nil)
	res, err := c.Do(context.Background(), httpconn.Request{
		URL:    upstream.URL + "/health",
		Method: "GET",
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.StatusCode != 200 {
		t.Fatalf("status=%d", res.StatusCode)
	}
	body, _ := res.Body.(map[string]interface{})
	if body["status"] != "ok" {
		t.Fatalf("body=%v", res.Body)
	}
}

func TestHTTPConnectorAPIAndURLMutuallyExclusive(t *testing.T) {
	c := httpconn.New(nil, nil)
	_, err := c.Do(context.Background(), httpconn.Request{
		API:    "demo",
		URL:    "http://example.com/x",
		Method: "GET",
		Path:   "/x",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
