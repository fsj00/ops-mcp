package openapi

import (
	"testing"

	"github.com/fsj00/ops-mcp/internal/model"
)

func TestDiscoveryIncludeExclude(t *testing.T) {
	disc, err := NewDiscoveryMatcher(model.APIDiscovery{
		Include: []model.DiscoveryRule{
			{OperationIDs: []string{"^get.*", "^list.*"}},
			{Methods: []string{"GET"}, Paths: []string{"/internal/*"}},
		},
		Exclude: []model.DiscoveryRule{
			{Methods: []string{"POST", "PUT", "PATCH", "DELETE"}},
			{OperationIDs: []string{".*Password.*"}},
			{Tags: []string{"admin"}},
			{Paths: []string{"/internal/debug/*"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		op   Operation
		want bool
	}{
		{Operation{OperationID: "getHost", Method: "GET", Path: "/host/{id}"}, true},
		{Operation{OperationID: "listHosts", Method: "GET", Path: "/hosts"}, true},
		{Operation{OperationID: "createHost", Method: "POST", Path: "/hosts"}, false},
		{Operation{OperationID: "getPassword", Method: "GET", Path: "/pwd"}, false},
		{Operation{OperationID: "doSomething", Method: "GET", Path: "/internal/status"}, true},
		{Operation{OperationID: "doSomething", Method: "GET", Path: "/internal/debug/x"}, false},
		{Operation{OperationID: "getHost", Method: "GET", Path: "/host", Tags: []string{"admin"}}, false},
		{Operation{OperationID: "updateHost", Method: "GET", Path: "/host"}, false}, // not in include
	}
	for _, tc := range cases {
		op := tc.op
		if got := disc.Match(&op); got != tc.want {
			t.Fatalf("op=%+v got=%v want=%v", tc.op, got, tc.want)
		}
	}
}

func TestEmptyIncludeMeansAll(t *testing.T) {
	disc, err := NewDiscoveryMatcher(model.APIDiscovery{
		Exclude: []model.DiscoveryRule{
			{Methods: []string{"DELETE"}},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	get := &Operation{OperationID: "x", Method: "GET", Path: "/a"}
	del := &Operation{OperationID: "y", Method: "DELETE", Path: "/a"}
	if !disc.Match(get) {
		t.Fatal("GET should be included")
	}
	if disc.Match(del) {
		t.Fatal("DELETE should be excluded")
	}
}

func TestPathMatchModes(t *testing.T) {
	cases := []struct {
		pattern, path string
		want          bool
	}{
		{"/api/v1/host", "/api/v1/host", true},
		{"/api/v1/host", "/api/v1/hosts", false},
		{"/host/{id}", "/host/{id}", true},
		{"/host/{id}", "/host/123", true},
		{"/host/123", "/host/{id}", true},
		{"/internal/*", "/internal/debug/x", true},
		{"/internal/*", "/other", false},
		{"^/api/v[0-9]+/host$", "/api/v1/host", true},
		{"^/api/v[0-9]+/host$", "/api/v1/hosts", false},
	}
	for _, tc := range cases {
		got, err := matchPath(tc.pattern, tc.path)
		if err != nil {
			t.Fatalf("pattern %q: %v", tc.pattern, err)
		}
		if got != tc.want {
			t.Fatalf("pattern=%q path=%q got=%v want=%v", tc.pattern, tc.path, got, tc.want)
		}
	}
}
