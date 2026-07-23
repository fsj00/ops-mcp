package runtime

import (
	"context"

	cmdconn "github.com/fsj00/ops-mcp/internal/connector/command"
	"github.com/fsj00/ops-mcp/internal/model"
	"github.com/dop251/goja"
)

func (r *Runtime) wrapCommandsList(vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Cfg == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "config manager not configured")))
		}
		return vm.ToValue(r.deps.Cfg.ListCommandSummaries())
	}
}

func (r *Runtime) wrapCommandExec(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		if r.deps.Command == nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "command connector not configured")))
		}
		var req cmdconn.ExecRequest
		if err := mapToStruct(call.Argument(0).Export(), &req); err != nil {
			panic(appErrorValue(vm, model.NewAppError(model.ErrInvalidParams, err.Error())))
		}
		res, err := r.deps.Command.Exec(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}
