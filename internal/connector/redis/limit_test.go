package redis

import (
	"testing"

	"github.com/fsj00/ops-mcp/internal/config"
)

func TestClampLimitRequired(t *testing.T) {
	if _, err := ClampLimit(0, 1000); err == nil {
		t.Fatal("expected error for limit=0")
	}
	if _, err := ClampLimit(-1, 1000); err == nil {
		t.Fatal("expected error for negative limit")
	}
}

func TestClampLimitOK(t *testing.T) {
	got, err := ClampLimit(100, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got != 100 {
		t.Fatalf("got %d", got)
	}
}

func TestClampLimitCap(t *testing.T) {
	got, err := ClampLimit(5000, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if got != 1000 {
		t.Fatalf("got %d want 1000", got)
	}
}

func TestClampLimitDefaultMax(t *testing.T) {
	got, err := ClampLimit(2000, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != config.DefaultQueryLimit {
		t.Fatalf("got %d want %d", got, config.DefaultQueryLimit)
	}
}
