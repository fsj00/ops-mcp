package executor_test

import (
	"context"
	"testing"
	"time"

	"github.com/fsj00/ops-mcp/internal/executor"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/fsj00/ops-mcp/internal/runtime"
)

func TestValidateRequired(t *testing.T) {
	rt := runtime.New(runtime.Dependencies{})
	ex := executor.New(rt, time.Second)
	p := &model.PluginMeta{
		Name:    "t",
		Script:  `function execute(ctx) { return {ok:true}; }`,
		Timeout: "1s",
		Input: map[string]model.PluginInputField{
			"host": {Type: "string", Required: true},
		},
	}
	res := ex.Execute(context.Background(), p, map[string]interface{}{})
	if res.Success {
		t.Fatal("expected failure")
	}
	if res.Error.Code != model.ErrInvalidParams {
		t.Fatalf("code=%s", res.Error.Code)
	}
}

func TestExecuteSuccess(t *testing.T) {
	rt := runtime.New(runtime.Dependencies{})
	ex := executor.New(rt, time.Second)
	p := &model.PluginMeta{
		Name:    "t",
		Script:  `function execute(ctx) { return { host: ctx.params.host }; }`,
		Timeout: "1s",
		Input: map[string]model.PluginInputField{
			"host": {Type: "string", Required: true},
		},
	}
	res := ex.Execute(context.Background(), p, map[string]interface{}{"host": "dev-ssh-111"})
	if !res.Success {
		t.Fatalf("fail: %+v", res.Error)
	}
	data, _ := res.Data.(map[string]interface{})
	if data["host"] != "dev-ssh-111" {
		t.Fatalf("data=%v", data)
	}
}
