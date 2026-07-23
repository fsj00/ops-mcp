package sqlguard

import (
	"strings"
	"testing"
)

func TestEnsureSelectOK(t *testing.T) {
	out, err := EnsureSelect("SELECT 1", 100)
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT * FROM (SELECT 1) AS _ops_mcp_q LIMIT 100"
	if out != want {
		t.Fatalf("got %q want %q", out, want)
	}
}

func TestEnsureSelectUnion(t *testing.T) {
	_, err := EnsureSelect("SELECT 1 UNION SELECT 2", 10)
	if err != nil {
		t.Fatal(err)
	}
}

func TestEnsureSelectRejectWrite(t *testing.T) {
	cases := []string{
		"INSERT INTO t VALUES (1)",
		"UPDATE t SET a=1",
		"DELETE FROM t",
		"DROP TABLE t",
		"SELECT 1; SELECT 2",
	}
	for _, sql := range cases {
		if _, err := EnsureSelect(sql, 100); err == nil {
			t.Fatalf("expected reject for %q", sql)
		}
	}
}

func TestEnsureSelectDefaultLimit(t *testing.T) {
	out, err := EnsureSelect("SELECT id FROM t", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "LIMIT 1000") {
		t.Fatalf("expected default limit: %s", out)
	}
}
