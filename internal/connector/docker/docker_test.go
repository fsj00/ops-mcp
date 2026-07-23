package docker

import (
	"testing"

	"github.com/fsj00/ops-mcp/internal/model"
)

func TestRequireHost(t *testing.T) {
	c := New(nil, nil)
	_, err := c.PS(t.Context(), PSRequest{})
	if err == nil {
		t.Fatal("expected host required error")
	}
	ae, ok := err.(*model.AppError)
	if !ok || ae.Code != model.ErrInvalidParams {
		t.Fatalf("got %v", err)
	}
}

func TestParseDockerPSJSON(t *testing.T) {
	in := `{"ID":"abc","Names":"web","Image":"nginx","Status":"Up","State":"running","Ports":"80/tcp","CreatedAt":"2026"}` + "\n"
	out, err := parseDockerPSJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].ID != "abc" || out[0].Names[0] != "web" {
		t.Fatalf("%+v", out)
	}
}

func TestParseDockerStatsJSON(t *testing.T) {
	in := `{"Container":"c1","Name":"web","CPUPerc":"1.2%","MemUsage":"10MiB / 1GiB"}` + "\n"
	out, err := parseDockerStatsJSON(in)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Container != "c1" || out[0].CPUPerc != "1.2%" {
		t.Fatalf("%+v", out)
	}
}
