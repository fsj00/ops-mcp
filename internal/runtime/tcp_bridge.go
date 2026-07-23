package runtime

import (
	"context"

	"github.com/dop251/goja"
	"github.com/fsj00/ops-mcp/internal/connector/netutil"
	tcpconn "github.com/fsj00/ops-mcp/internal/connector/tcp"
	"github.com/fsj00/ops-mcp/internal/model"
)

func (r *Runtime) tcpConn(vm *goja.Runtime) *tcpconn.Connector {
	if r.deps.TCP == nil {
		panic(appErrorValue(vm, model.NewAppError(model.ErrConnectorError, "tcp connector not configured")))
	}
	return r.deps.TCP
}

func (r *Runtime) wrapTCPExchange(ctx context.Context, vm *goja.Runtime) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		req := decodeArg[netutil.ExchangeRequest](vm, call)
		res, err := r.tcpConn(vm).Exchange(ctx, req)
		if err != nil {
			panic(appErrorValue(vm, err))
		}
		return vm.ToValue(res)
	}
}
